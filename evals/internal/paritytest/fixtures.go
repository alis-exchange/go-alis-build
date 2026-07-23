package paritytest

import (
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/mapper"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func float64Ptr(v float64) *float64 { return &v }
func stringPtr(v string) *string    { return &v }

// IntegrationBaselineRun materializes the current mapper output for a representative
// integration workflow with passed, failed, and not-evaluated cases.
func IntegrationBaselineRun() *evalspb.Run {
	meta := DefaultFixedRunMeta
	sr := execution.SuiteResult{
		SuiteName: "checkout-regression",
		StartTime: meta.StartTime,
		EndTime:   meta.EndTime,
		Cases: []execution.CaseResult{
			{
				Name:     "checkout-regression.creates-order",
				Status:   evalspb.Status_PASSED,
				Duration: 1500 * time.Millisecond,
				Checks: []execution.Check{
					{ID: "grpc.status_ok", Status: evalspb.Status_PASSED},
					{ID: "order.id_present", Status: evalspb.Status_PASSED},
				},
			},
			{
				Name:     "checkout-regression.latency-budget",
				Status:   evalspb.Status_FAILED,
				Duration: 2200 * time.Millisecond,
				Checks: []execution.Check{
					{ID: "grpc.status_ok", Status: evalspb.Status_PASSED},
					{ID: "latency.p99_ms", Status: evalspb.Status_FAILED, Message: "p99 842ms exceeds limit 500ms"},
				},
			},
			{
				Name:     "checkout-regression.cancelled-case",
				Status:   evalspb.Status_NOT_EVALUATED,
				Duration: 0,
				Checks: []execution.Check{{
					ID:      "_evals.skipped",
					Status:  evalspb.Status_NOT_EVALUATED,
					Message: "run cancelled",
				}},
			},
		},
	}
	return NormalizeRun(
		mapper.IntegrationRun(sr, meta.Operation, meta.RunID, meta.BatchID),
		meta,
	)
}

// AgentBaselineRun materializes the current mapper output for a representative
// agent-eval workflow with judge provenance, rubric scores, and mixed outcomes.
func AgentBaselineRun() *evalspb.Run {
	meta := DefaultFixedRunMeta
	rubricPass := 0.85
	rubricFail := 0.42
	sr := execution.SuiteResult{
		SuiteName: "assistant-quality",
		StartTime: meta.StartTime,
		EndTime:   meta.EndTime,
		Judge: execution.JudgeInfo{
			Model:        "gemini-2.5-pro",
			ModelVersion: "2025-06-05",
		},
		JudgeCallCount: 3,
		Cases: []execution.CaseResult{
			{
				Name:           "assistant-quality.answers-correctly",
				Status:         evalspb.Status_PASSED,
				SessionID:      "sessions/agent-baseline-1",
				Duration:       time.Second,
				JudgeCallCount: 2,
				Metrics: []execution.Metric{
					{
						ID:        "rubric_based_final_response_quality_v1",
						Status:    evalspb.Status_PASSED,
						Threshold: 0.7,
						Score:     &rubricPass,
						Rubric: []execution.RubricScore{
							{ID: "accuracy", Status: evalspb.Status_PASSED, Score: float64Ptr(0.9), Rationale: "answer matched reference"},
						},
					},
					{ID: "hallucinations_v1", Status: evalspb.Status_PASSED, Threshold: 0.8, Score: float64Ptr(0.95)},
				},
			},
			{
				Name:           "assistant-quality.cites-source",
				Status:         evalspb.Status_FAILED,
				SessionID:      "sessions/agent-baseline-2",
				Duration:       2 * time.Second,
				JudgeCallCount: 1,
				Metrics: []execution.Metric{
					{
						ID:        "rubric_based_final_response_quality_v1",
						Status:    evalspb.Status_FAILED,
						Message:   "score 0.42 below threshold 0.70",
						Threshold: 0.7,
						Score:     &rubricFail,
						Rubric: []execution.RubricScore{
							{
								ID:        "citation",
								Status:    evalspb.Status_FAILED,
								Score:     &rubricFail,
								Rationale: "response omitted the required source citation",
							},
						},
					},
				},
			},
			{
				Name:     "assistant-quality.cancelled-case",
				Status:   evalspb.Status_NOT_EVALUATED,
				Duration: 0,
				Metrics: []execution.Metric{{
					ID:      "_evals.skipped",
					Status:  evalspb.Status_NOT_EVALUATED,
					Message: "run cancelled",
				}},
			},
		},
	}
	return NormalizeRun(
		mapper.AgentEvalRun(sr, meta.Operation, meta.RunID),
		meta,
	)
}

// LoadBaselineRun materializes the current mapper output for a representative
// load workflow including summary, SLO checks, tags, streaming metrics, and infra snapshots.
func LoadBaselineRun() *evalspb.Run {
	meta := DefaultFixedRunMeta
	sr := execution.LoadSuiteResult{
		SuiteName: "checkout-capacity",
		StartTime: meta.StartTime,
		EndTime:   meta.EndTime,
		Cases: []execution.LoadCaseResult{
			{
				Name:   "checkout-capacity.steady-traffic",
				Status: evalspb.Status_FAILED,
				Tags:   map[string]string{"rpc": "CreateOrder", "env": "staging"},
				Summary: execution.LoadCaseSummary{
					Mode:             evalspb.RunLoadTestRequest_MODERATE,
					TargetQPS:        100,
					Concurrency:      25,
					Duration:         time.Second,
					RequestCount:     95,
					ErrorCount:       2,
					CheckPassedCount: 90,
					CheckFailedCount: 3,
					DroppedCount:     5,
					ActualQPS:        95,
					QPSStages:        []execution.LoadStage{{Duration: time.Second, Target: 100}},
					ConcurrencyStages: []execution.LoadStage{
						{Duration: 500 * time.Millisecond, Target: 10},
						{Duration: 500 * time.Millisecond, Target: 25},
					},
					Latency: execution.LoadLatency{
						P50Ms: 8, P95Ms: 60, P99Ms: 120, MinMs: 2, MeanMs: 12, MaxMs: 250,
					},
					ErrorsByCode: map[string]int64{"UNAVAILABLE": 2},
					Stream: &execution.LoadStreamSummary{
						StreamCount:       10,
						MessagesSentTotal: 40,
						TTFB:              execution.LoadLatency{P99Ms: 25},
						ResponseLatency:   execution.LoadLatency{P50Ms: 15, P99Ms: 80},
						TotalDuration:     execution.LoadLatency{P50Ms: 20, P99Ms: 100, MaxMs: 140},
					},
				},
				Checks: []execution.SloCheckResult{
					{ID: "latency.p99_ms", Status: evalspb.Status_PASSED, Observed: 120, Limit: 500, Unit: "ms"},
					{ID: "error_rate", Status: evalspb.Status_FAILED, Observed: 2.1, Limit: 1.0, Unit: "%", Message: "2.1% exceeds limit 1.0%"},
				},
				CloudRun: []*evalspb.CloudRunTargetSnapshot{{
					Id:   "checkout-entry",
					Role: evalspb.InfraTargetRole_INFRA_TARGET_ROLE_ENTRY,
					Target: &evalspb.CloudRunTargetRef{
						ProjectId:   "marvel-sm-dev-123",
						Region:      "africa-south1",
						ServiceName: "checkout",
						Revision:    stringPtr("checkout-00042"),
					},
					WindowStart:  timestamppb.New(meta.StartTime),
					WindowEnd:    timestamppb.New(meta.EndTime),
					FetchStatus:  evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK,
					FetchMessage: stringPtr("all Cloud Run metrics fetched"),
					Metrics: &evalspb.CloudRunMetrics{
						RequestCount:         12,
						Latency:              &evalspb.LatencyPercentiles{P50Ms: 8, P95Ms: 60, P99Ms: 120, MinMs: 2, MeanMs: 12, MaxMs: 250},
						Error_5XxRate:        float64Ptr(0.01),
						MaxInstanceCount:     float64Ptr(3),
						CpuUtilizationP99:    float64Ptr(0.72),
						MemoryUtilizationP99: float64Ptr(0.64),
						StartupLatencyP99:    float64Ptr(420),
					},
				}},
				Spanner: []*evalspb.SpannerTargetSnapshot{{
					Id:   "checkout-db",
					Role: evalspb.InfraTargetRole_INFRA_TARGET_ROLE_DEPENDENCY,
					Target: &evalspb.SpannerTargetRef{
						ProjectId:  "marvel-sm-dev-123",
						InstanceId: "checkout",
						Location:   "regional-europe-west1",
						Database:   "orders",
					},
					WindowStart:  timestamppb.New(meta.StartTime),
					WindowEnd:    timestamppb.New(meta.EndTime),
					FetchStatus:  evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK,
					FetchMessage: stringPtr("all Spanner metrics fetched"),
					Metrics: &evalspb.SpannerMetrics{
						QueryCount:        4,
						QueryErrorCount:   1,
						ApiLatency:        &evalspb.LatencyPercentiles{P50Ms: 4, P99Ms: 20},
						QueryLatency:      &evalspb.LatencyPercentiles{P50Ms: 3, P99Ms: 18},
						CpuUtilizationMax: float64Ptr(0.42),
					},
				}},
			},
		},
	}
	return NormalizeRun(
		mapper.LoadRun(sr, meta.Operation, meta.RunID, meta.BatchID),
		meta,
	)
}

// InfraObservationBaselineRun materializes the current mapper output for a representative
// infra-observation workflow with lookback, windows, and mixed fetch statuses.
func InfraObservationBaselineRun() *evalspb.Run {
	meta := DefaultFixedRunMeta
	windowStart := meta.StartTime
	windowEnd := meta.EndTime
	sr := execution.InfraObserveSuiteResult{
		SuiteName: "checkout-runtime",
		StartTime: meta.StartTime,
		EndTime:   meta.EndTime,
		Cases: []execution.InfraObserveCaseResult{
			{
				Name:        "checkout-runtime.peak-window",
				Status:      evalspb.Status_PASSED,
				Lookback:    30 * time.Minute,
				WindowStart: windowStart,
				WindowEnd:   windowEnd,
				CloudRun: []*evalspb.CloudRunTargetSnapshot{{
					Id:   "checkout-entry",
					Role: evalspb.InfraTargetRole_INFRA_TARGET_ROLE_ENTRY,
					Target: &evalspb.CloudRunTargetRef{
						ProjectId:   "marvel-sm-prod-456",
						Region:      "africa-south1",
						ServiceName: "checkout",
						Revision:    stringPtr("checkout-00100"),
					},
					WindowStart:  timestamppb.New(windowStart),
					WindowEnd:    timestamppb.New(windowEnd),
					FetchStatus:  evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK,
					FetchMessage: stringPtr("observation complete"),
					Metrics: &evalspb.CloudRunMetrics{
						RequestCount:         99,
						Latency:              &evalspb.LatencyPercentiles{P50Ms: 12, P95Ms: 55, P99Ms: 110, MinMs: 3, MeanMs: 18, MaxMs: 210},
						Error_5XxRate:        float64Ptr(0.02),
						MaxInstanceCount:     float64Ptr(4),
						CpuUtilizationP99:    float64Ptr(0.68),
						MemoryUtilizationP99: float64Ptr(0.59),
						StartupLatencyP99:    float64Ptr(390),
					},
				}},
				Spanner: []*evalspb.SpannerTargetSnapshot{{
					Id:   "checkout-db",
					Role: evalspb.InfraTargetRole_INFRA_TARGET_ROLE_DEPENDENCY,
					Target: &evalspb.SpannerTargetRef{
						ProjectId:  "marvel-sm-prod-456",
						InstanceId: "checkout",
						Location:   "regional-europe-west1",
						Database:   "orders",
					},
					WindowStart:  timestamppb.New(windowStart),
					WindowEnd:    timestamppb.New(windowEnd),
					FetchStatus:  evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK,
					FetchMessage: stringPtr("observation complete"),
					Metrics: &evalspb.SpannerMetrics{
						QueryCount:        18,
						QueryErrorCount:   2,
						ApiLatency:        &evalspb.LatencyPercentiles{P50Ms: 5, P99Ms: 24},
						QueryLatency:      &evalspb.LatencyPercentiles{P50Ms: 4, P99Ms: 21},
						CpuUtilizationMax: float64Ptr(0.55),
					},
				}},
			},
			{
				Name:        "checkout-runtime.partial-fetch",
				Status:      evalspb.Status_FAILED,
				Lookback:    15 * time.Minute,
				WindowStart: windowStart,
				WindowEnd:   windowEnd,
				CloudRun: []*evalspb.CloudRunTargetSnapshot{{
					Id:   "checkout-worker",
					Role: evalspb.InfraTargetRole_INFRA_TARGET_ROLE_DEPENDENCY,
					Target: &evalspb.CloudRunTargetRef{
						ProjectId:   "marvel-sm-prod-456",
						Region:      "africa-south1",
						ServiceName: "checkout-worker",
					},
					WindowStart:  timestamppb.New(windowStart),
					WindowEnd:    timestamppb.New(windowEnd),
					FetchStatus:  evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE,
					FetchMessage: stringPtr("Monitoring API returned no usable data"),
					Metrics:      nil,
				}},
			},
		},
	}
	return NormalizeRun(
		mapper.InfraObserveRun(sr, meta.Operation, meta.RunID, meta.BatchID),
		meta,
	)
}
