package evals

import (
	"context"
	"errors"
	"reflect"
	"testing"

	evalspb "go.alis.build/common/alis/evals"
	"google.golang.org/protobuf/proto"
)

func TestAgentEvalResult_buildsCaseWithSessionMetricsAndValidations(t *testing.T) {
	t.Parallel()

	score := 0.83
	threshold := 0.8
	metric := &evalspb.AgentEvalResults_Case_Metric{
		Id:        "answer_quality",
		Status:    evalspb.Status_PASSED,
		Score:     &score,
		Threshold: &threshold,
		Message:   "usable",
	}

	run, err := NewAgentEvalSuite("agent-builder").
		AddCase("answers", func(_ context.Context, r *AgentEvalResult) {
			if r.Validator() == nil {
				t.Fatal("Validator() returned nil")
			}
			r.SetSessionID("sessions/s-1")
			r.Validator().Custom("answer present", true)
			r.Validator().Custom("citation present", false)
			r.AddMetric(metric)
			metric.Id = "mutated-after-add"
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cases := run.GetAgentEval().GetCases()
	if len(cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(cases))
	}
	c := cases[0]
	if c.GetId() != "agent-builder.answers" {
		t.Fatalf("case id = %q, want qualified id", c.GetId())
	}
	if c.GetSessionId() != "sessions/s-1" {
		t.Fatalf("session_id = %q, want sessions/s-1", c.GetSessionId())
	}
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
	if run.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("run status = %v, want FAILED", run.GetStatus())
	}
	if len(c.GetMetrics()) != 1 {
		t.Fatalf("metrics = %d, want 1", len(c.GetMetrics()))
	}
	if c.GetMetrics()[0].GetId() != "answer_quality" {
		t.Fatalf("metric id = %q, want cloned source value", c.GetMetrics()[0].GetId())
	}
	gotValidations := validationTriples(c.GetValidations())
	wantValidations := []validationTriple{
		{id: "answer present", status: evalspb.Status_PASSED},
		{id: "citation present", status: evalspb.Status_FAILED, message: "citation present"},
	}
	if !reflect.DeepEqual(gotValidations, wantValidations) {
		t.Fatalf("validations = %+v, want %+v", gotValidations, wantValidations)
	}
}

func TestAgentEvalResult_failPreservesPartialData(t *testing.T) {
	t.Parallel()

	run, err := NewAgentEvalSuite("agent-fail").
		AddCase("partial", func(_ context.Context, r *AgentEvalResult) {
			r.SetSessionID("sessions/s-2")
			r.AddMetric(&evalspb.AgentEvalResults_Case_Metric{Id: "fluency", Status: evalspb.Status_PASSED})
			r.Fail(errors.New("judge service unavailable"))
			r.Fail(nil)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetAgentEval().GetCases()[0]
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
	if c.GetSessionId() != "sessions/s-2" || len(c.GetMetrics()) != 1 {
		t.Fatalf("partial data not preserved: %+v", c)
	}
	got := validationTriples(c.GetValidations())
	want := []validationTriple{{id: "_evals.case", status: evalspb.Status_FAILED, message: "judge service unavailable"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("validations = %+v, want %+v", got, want)
	}
}

func TestAgentEvalResult_emptyCaseIsNotEvaluated(t *testing.T) {
	t.Parallel()

	run, err := NewAgentEvalSuite("agent-empty").
		AddCase("empty", noopAgentCase).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetAgentEval().GetCases()[0]
	if c.GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("case status = %v, want NOT_EVALUATED", c.GetStatus())
	}
	if run.GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("run status = %v, want NOT_EVALUATED", run.GetStatus())
	}
}

func TestAgentEvalResult_metricFailureFailsCase(t *testing.T) {
	t.Parallel()

	run, err := NewAgentEvalSuite("agent-metric").
		AddCase("metric-fails", func(_ context.Context, r *AgentEvalResult) {
			r.AddMetric(&evalspb.AgentEvalResults_Case_Metric{Id: "groundedness", Status: evalspb.Status_FAILED})
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetAgentEval().GetCases()[0]
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
}

func TestAgentEvalResult_aggregatesJudgeInfo(t *testing.T) {
	t.Parallel()

	modelVersion := "2026-07-23"
	run, err := NewAgentEvalSuite("agent-judge").
		AddCase("first", func(_ context.Context, r *AgentEvalResult) {
			r.SetJudgeInfo(&evalspb.AgentEvalResults_JudgeInfo{
				Model:           "gemini-2.5-pro",
				ModelVersion:    &modelVersion,
				JudgeCallCount:  2,
				JudgeErrorCount: 1,
			})
		}).
		AddCase("second", func(_ context.Context, r *AgentEvalResult) {
			r.SetJudgeInfo(&evalspb.AgentEvalResults_JudgeInfo{
				Model:          "gemini-2.5-pro",
				ModelVersion:   &modelVersion,
				JudgeCallCount: 3,
			})
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	judge := run.GetAgentEval().GetJudge()
	if judge == nil {
		t.Fatal("judge = nil, want populated")
	}
	if judge.GetModel() != "gemini-2.5-pro" {
		t.Fatalf("judge model = %q, want gemini-2.5-pro", judge.GetModel())
	}
	if judge.GetModelVersion() != modelVersion {
		t.Fatalf("judge model version = %q, want %q", judge.GetModelVersion(), modelVersion)
	}
	if judge.GetJudgeCallCount() != 5 {
		t.Fatalf("judge call count = %d, want 5", judge.GetJudgeCallCount())
	}
	if judge.GetJudgeErrorCount() != 1 {
		t.Fatalf("judge error count = %d, want 1", judge.GetJudgeErrorCount())
	}
}

func TestAgentEvalResult_conflictingJudgeInfoFailsOnlyConflictingCase(t *testing.T) {
	t.Parallel()

	run, err := NewAgentEvalSuite("agent-judge-conflict").
		AddCase("first", func(_ context.Context, r *AgentEvalResult) {
			r.SetJudgeInfo(&evalspb.AgentEvalResults_JudgeInfo{
				Model:          "gemini-2.5-pro",
				JudgeCallCount: 2,
			})
		}).
		AddCase("second", func(_ context.Context, r *AgentEvalResult) {
			r.SetJudgeInfo(&evalspb.AgentEvalResults_JudgeInfo{
				Model:          "gemini-2.5-flash",
				JudgeCallCount: 3,
			})
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cases := run.GetAgentEval().GetCases()
	if cases[0].GetStatus() != evalspb.Status_PASSED {
		t.Fatalf("first case status = %v, want PASSED", cases[0].GetStatus())
	}
	if cases[1].GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("conflicting case status = %v, want FAILED", cases[1].GetStatus())
	}
	validations := cases[1].GetValidations()
	if len(validations) != 1 || validations[0].GetId() != "_evals.judge" {
		t.Fatalf("conflict validations = %+v, want _evals.judge failure", validations)
	}
	judge := run.GetAgentEval().GetJudge()
	if judge.GetModel() != "gemini-2.5-pro" {
		t.Fatalf("judge model = %q, want first declaration", judge.GetModel())
	}
	if judge.GetJudgeCallCount() != 2 {
		t.Fatalf("judge call count = %d, want conflicting case excluded", judge.GetJudgeCallCount())
	}
	if run.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("run status = %v, want FAILED", run.GetStatus())
	}
}

func TestAgentEvalResult_omitsZeroJudgeAndClonesJudgeInfo(t *testing.T) {
	t.Parallel()

	version := "v1"
	judge := &evalspb.AgentEvalResults_JudgeInfo{
		Model:          "gemini-2.5-pro",
		ModelVersion:   &version,
		JudgeCallCount: 1,
	}
	run, err := NewAgentEvalSuite("agent-clone").
		AddCase("with-judge", func(_ context.Context, r *AgentEvalResult) {
			r.SetJudgeInfo(judge)
			judge.Model = "mutated"
			*judge.ModelVersion = "mutated"
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := run.GetAgentEval().GetJudge()
	if got.GetModel() != "gemini-2.5-pro" || got.GetModelVersion() != "v1" {
		t.Fatalf("judge was not cloned: %+v", got)
	}

	empty, err := NewAgentEvalSuite("agent-no-judge").
		AddCase("case", func(_ context.Context, r *AgentEvalResult) {
			r.SetJudgeInfo(&evalspb.AgentEvalResults_JudgeInfo{})
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if empty.GetAgentEval().GetJudge() != nil {
		t.Fatalf("judge = %+v, want nil for zero-valued judge info", empty.GetAgentEval().GetJudge())
	}
}

func TestAgentEvalResult_nilAndDuplicateBuilderInputsBecomeValidations(t *testing.T) {
	t.Parallel()

	run, err := NewAgentEvalSuite("agent-builder-errors").
		AddCase("bad-inputs", func(_ context.Context, r *AgentEvalResult) {
			r.SetSessionID("first")
			r.SetSessionID("second")
			r.AddMetric(nil)
			r.SetJudgeInfo(nil)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetAgentEval().GetCases()[0]
	if c.GetSessionId() != "first" {
		t.Fatalf("session_id = %q, want first value retained", c.GetSessionId())
	}
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
	gotMessages := []string{}
	for _, v := range c.GetValidations() {
		gotMessages = append(gotMessages, v.GetMessage())
	}
	wantMessages := []string{
		"evals: agent session id already set",
		"evals: nil agent metric",
		"evals: nil agent judge info",
	}
	if !reflect.DeepEqual(gotMessages, wantMessages) {
		t.Fatalf("validation messages = %v, want %v", gotMessages, wantMessages)
	}
}

func TestAgentEvalResult_usesProtobufNativeMetricValues(t *testing.T) {
	t.Parallel()

	rationale := "clear evidence"
	score := 0.91
	metric := &evalspb.AgentEvalResults_Case_Metric{
		Id:      "rubric",
		Status:  evalspb.Status_PASSED,
		Score:   &score,
		Message: "ok",
		Rubric: []*evalspb.AgentEvalResults_Case_Metric_RubricScore{{
			Id:        "accuracy",
			Status:    evalspb.Status_PASSED,
			Score:     &score,
			Rationale: &rationale,
		}},
	}
	run, err := NewAgentEvalSuite("agent-native").
		AddCase("metric", func(_ context.Context, r *AgentEvalResult) {
			r.AddMetric(metric)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := run.GetAgentEval().GetCases()[0].GetMetrics()[0]
	if !proto.Equal(got, metric) {
		t.Fatalf("metric = %v, want protobuf-native value %v", got, metric)
	}
}

type validationTriple struct {
	id      string
	status  evalspb.Status
	message string
}

func validationTriples(validations []*evalspb.Validation) []validationTriple {
	out := make([]validationTriple, len(validations))
	for i, v := range validations {
		out[i] = validationTriple{
			id:      v.GetId(),
			status:  v.GetStatus(),
			message: v.GetMessage(),
		}
	}
	return out
}

// TestAgentEvalResult_setNotEvaluated checks that SetNotEvaluated keeps data,
// records one NOT_EVALUATED validation per call, and loses to any failure.
func TestAgentEvalResult_setNotEvaluated(t *testing.T) {
	t.Parallel()

	const reason = "adk: final eval status NOT_EVALUATED"
	notEvaluated := func(msg string) validationTriple {
		return validationTriple{id: caseValidationID, status: evalspb.Status_NOT_EVALUATED, message: msg}
	}
	tests := []struct {
		name            string
		fn              AgentEvalCaseFunc
		wantStatus      evalspb.Status
		wantSessionID   string
		wantMetrics     int
		wantValidations []validationTriple
	}{
		{
			name: "keeps data",
			fn: func(_ context.Context, r *AgentEvalResult) {
				r.SetSessionID("sess-1")
				r.AddMetric(&evalspb.AgentEvalResults_Case_Metric{Id: "answer_quality", Status: evalspb.Status_PASSED})
				r.SetNotEvaluated(reason)
			},
			wantStatus:      evalspb.Status_NOT_EVALUATED,
			wantSessionID:   "sess-1",
			wantMetrics:     1,
			wantValidations: []validationTriple{notEvaluated(reason)},
		},
		{
			name:            "empty reason uses default",
			fn:              func(_ context.Context, r *AgentEvalResult) { r.SetNotEvaluated("") },
			wantStatus:      evalspb.Status_NOT_EVALUATED,
			wantValidations: []validationTriple{notEvaluated("evals: case not evaluated")},
		},
		{
			name: "twice records two validations",
			fn: func(_ context.Context, r *AgentEvalResult) {
				r.SetNotEvaluated("first")
				r.SetNotEvaluated("second")
			},
			wantStatus:      evalspb.Status_NOT_EVALUATED,
			wantValidations: []validationTriple{notEvaluated("first"), notEvaluated("second")},
		},
		{
			name: "failed metric wins",
			fn: func(_ context.Context, r *AgentEvalResult) {
				r.AddMetric(&evalspb.AgentEvalResults_Case_Metric{Id: "groundedness", Status: evalspb.Status_FAILED})
				r.SetNotEvaluated(reason)
			},
			wantStatus:      evalspb.Status_FAILED,
			wantMetrics:     1,
			wantValidations: []validationTriple{notEvaluated(reason)},
		},
		{
			name: "fail wins",
			fn: func(_ context.Context, r *AgentEvalResult) {
				r.SetNotEvaluated(reason)
				r.Fail(errors.New("boom"))
			},
			wantStatus: evalspb.Status_FAILED,
			wantValidations: []validationTriple{
				{id: caseValidationID, status: evalspb.Status_FAILED, message: "boom"},
				notEvaluated(reason),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			run, err := NewAgentEvalSuite("agent-not-evaluated").AddCase("case", tt.fn).Run(context.Background())
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			c := run.GetAgentEval().GetCases()[0]
			if c.GetStatus() != tt.wantStatus {
				t.Fatalf("case status = %v, want %v", c.GetStatus(), tt.wantStatus)
			}
			if c.GetSessionId() != tt.wantSessionID || len(c.GetMetrics()) != tt.wantMetrics {
				t.Fatalf("session = %q, metrics = %d; want %q, %d", c.GetSessionId(), len(c.GetMetrics()), tt.wantSessionID, tt.wantMetrics)
			}
			if got := validationTriples(c.GetValidations()); !reflect.DeepEqual(got, tt.wantValidations) {
				t.Fatalf("validations = %+v, want %+v", got, tt.wantValidations)
			}
		})
	}
}

// TestAgentEvalResult_setNotEvaluatedRollsUpRun checks a forced case makes
// the run NOT_EVALUATED next to a PASSED case, and FAILED still wins.
func TestAgentEvalResult_setNotEvaluatedRollsUpRun(t *testing.T) {
	t.Parallel()

	forced := func(_ context.Context, r *AgentEvalResult) { r.SetNotEvaluated("") }
	passed := func(_ context.Context, r *AgentEvalResult) { r.SetSessionID("sess-ok") }
	failed := func(_ context.Context, r *AgentEvalResult) { r.Fail(errors.New("boom")) }

	tests := []struct {
		name  string
		other AgentEvalCaseFunc
		want  evalspb.Status
	}{
		{name: "with passed", other: passed, want: evalspb.Status_NOT_EVALUATED},
		{name: "with failed", other: failed, want: evalspb.Status_FAILED},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			run, err := NewAgentEvalSuite("agent-not-evaluated-run").
				AddCase("forced", forced).
				AddCase("other", tt.other).
				Run(context.Background())
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if run.GetStatus() != tt.want {
				t.Fatalf("run status = %v, want %v", run.GetStatus(), tt.want)
			}
		})
	}
}
