package evals

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/validation"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	now     = time.Now
	newUUID = uuid.NewString
)

const panicCheckID = "_evals.panic"
const skippedCheckID = "_evals.skipped"
const skippedMessage = "run cancelled"

const (
	caseValidationID  = "_evals.case"
	judgeValidationID = "_evals.judge"
)

func resolveGoogleProjectID(cfg runConfig) string {
	if cfg.googleProject != "" {
		return cfg.googleProject
	}
	return os.Getenv("ALIS_OS_PROJECT")
}

func qualifiedCaseID(suiteName, caseName string) string {
	return suiteName + "." + caseName
}

type executedCase struct {
	index       int
	name        string
	status      evalspb.Status
	duration    time.Duration
	checks      []*evalspb.IntegrationTestResults_Case_Check
	validations []*evalspb.Validation
	sessionID   string
	metrics     []*evalspb.AgentEvalResults_Case_Metric
	judge       *evalspb.AgentEvalResults_JudgeInfo
	judgeSkip   bool
	summary     *evalspb.LoadTestResults_Summary
	loadChecks  []*evalspb.LoadTestResults_SloCheck
	tags        []*evalspb.LoadTestResults_StringEntry
	cloudRun    []*evalspb.CloudRunTargetSnapshot
	spanner     []*evalspb.SpannerTargetSnapshot
	infraChecks []*evalspb.InfraSloCheck
	lookback    time.Duration
	windowStart time.Time
	windowEnd   time.Time
	windowSet   bool
}

func (s *suiteCore) executeCases(ctx context.Context, cfg runConfig, registered []registeredCase) []executedCase {
	cases := make([]executedCase, len(registered))
	if len(registered) == 0 {
		return cases
	}
	for i, c := range registered {
		cases[i] = executedCase{
			index:  i,
			name:   c.name,
			status: evalspb.Status_NOT_EVALUATED,
		}
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := min(cfg.maxConcurrency, len(registered))
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				fc := registered[idx]
				start := now()
				out := s.runOneCaseRecovering(ctx, fc)
				out.duration = now().Sub(start)
				out.index = idx
				out.name = fc.name
				cases[idx] = out
			}
		}()
	}

	for i := range registered {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			s.markCancelledCases(cases)
			return cases
		case jobs <- i:
		}
	}
	close(jobs)

	wg.Wait()
	if ctx.Err() != nil {
		s.markCancelledCases(cases)
	}
	return cases
}

func (s *suiteCore) markCancelledCases(cases []executedCase) {
	for i := range cases {
		if cases[i].status != evalspb.Status_NOT_EVALUATED || hasCaseResultData(cases[i]) {
			continue
		}
		switch s.branch {
		case branchIntegration:
			cases[i].checks = []*evalspb.IntegrationTestResults_Case_Check{{
				Id:      skippedCheckID,
				Status:  evalspb.Status_NOT_EVALUATED,
				Message: skippedMessage,
			}}
		case branchAgentEval:
			cases[i].metrics = []*evalspb.AgentEvalResults_Case_Metric{{
				Id:      skippedCheckID,
				Status:  evalspb.Status_NOT_EVALUATED,
				Message: skippedMessage,
			}}
		case branchLoad, branchInfraObservation:
			cases[i].validations = []*evalspb.Validation{{
				Id:      skippedCheckID,
				Status:  evalspb.Status_NOT_EVALUATED,
				Message: skippedMessage,
			}}
		}
	}
}

func hasCaseResultData(c executedCase) bool {
	return len(c.checks) > 0 ||
		len(c.validations) > 0 ||
		c.sessionID != "" ||
		len(c.metrics) > 0 ||
		c.judge != nil ||
		c.summary != nil ||
		len(c.loadChecks) > 0 ||
		len(c.tags) > 0 ||
		len(c.cloudRun) > 0 ||
		len(c.spanner) > 0 ||
		len(c.infraChecks) > 0 ||
		c.windowSet
}

func (s *suiteCore) runOneCaseRecovering(ctx context.Context, fc registeredCase) (out executedCase) {
	defer func() {
		if panicValue := recover(); panicValue != nil {
			out = s.panicOutcome(fc.name, panicValue)
		}
	}()
	return s.runOneCase(ctx, fc)
}

func (s *suiteCore) panicOutcome(caseName string, panicValue any) executedCase {
	message := fmt.Sprintf("panic: %v", panicValue)
	if s.branch == branchIntegration {
		return executedCase{
			name:   caseName,
			status: evalspb.Status_FAILED,
			checks: []*evalspb.IntegrationTestResults_Case_Check{{
				Id:      panicCheckID,
				Status:  evalspb.Status_FAILED,
				Message: message,
			}},
		}
	}
	return executedCase{
		name:   caseName,
		status: evalspb.Status_FAILED,
		validations: []*evalspb.Validation{{
			Id:      panicCheckID,
			Status:  evalspb.Status_FAILED,
			Message: message,
		}},
	}
}

func (s *suiteCore) runOneCase(ctx context.Context, fc registeredCase) executedCase {
	switch s.branch {
	case branchIntegration:
		return runIntegrationCase(ctx, s.name, fc)
	case branchAgentEval:
		return runAgentEvalCase(ctx, s.name, fc)
	case branchLoad:
		return runLoadCase(ctx, s.name, fc)
	case branchInfraObservation:
		return runInfraObservationCase(ctx, s.name, fc)
	default:
		return executedCase{status: evalspb.Status_FAILED}
	}
}

func runIntegrationCase(ctx context.Context, suiteName string, fc registeredCase) executedCase {
	fn, _ := fc.fn.(IntegrationCaseFunc)
	v := validation.NewValidator()
	if fn != nil {
		fn(ctx, v)
	}
	return integrationOutcome(suiteName, fc.name, v, 0)
}

func integrationOutcome(suiteName, caseName string, v *validation.Validator, dur time.Duration) executedCase {
	rules := v.Rules()
	checks := make([]*evalspb.IntegrationTestResults_Case_Check, 0, len(rules))
	if len(rules) == 0 {
		return executedCase{
			name:     caseName,
			status:   evalspb.Status_NOT_EVALUATED,
			duration: dur,
		}
	}
	status := evalspb.Status_PASSED
	for _, r := range rules {
		checkStatus := evalspb.Status_PASSED
		var msg string
		if !r.Satisfied() {
			checkStatus = evalspb.Status_FAILED
			status = evalspb.Status_FAILED
			msg = r.Rule()
		}
		checks = append(checks, &evalspb.IntegrationTestResults_Case_Check{
			Id:      r.Rule(),
			Status:  checkStatus,
			Message: msg,
		})
	}
	return executedCase{
		name:     caseName,
		status:   status,
		duration: dur,
		checks:   checks,
	}
}

func validationsFromValidator(v *validation.Validator) []*evalspb.Validation {
	rules := v.Rules()
	validations := make([]*evalspb.Validation, 0, len(rules))
	for _, r := range rules {
		status := evalspb.Status_PASSED
		var msg string
		if !r.Satisfied() {
			status = evalspb.Status_FAILED
			msg = r.Rule()
		}
		validations = append(validations, &evalspb.Validation{
			Id:      r.Rule(),
			Status:  status,
			Message: msg,
		})
	}
	return validations
}

func reconcileAgentJudge(cases []executedCase) {
	var declared *evalspb.AgentEvalResults_JudgeInfo
	for i := range cases {
		judge := cases[i].judge
		if !hasAgentJudgeData(judge) {
			continue
		}
		if declared == nil {
			declared = judge
			continue
		}
		if sameAgentJudgeDeclaration(declared, judge) {
			continue
		}
		cases[i].judgeSkip = true
		cases[i].status = evalspb.Status_FAILED
		cases[i].validations = append(cases[i].validations, &evalspb.Validation{
			Id:      judgeValidationID,
			Status:  evalspb.Status_FAILED,
			Message: "evals: agent judge model/version conflict",
		})
	}
}

func agentJudge(cases []executedCase) *evalspb.AgentEvalResults_JudgeInfo {
	var out *evalspb.AgentEvalResults_JudgeInfo
	for _, c := range cases {
		if c.judgeSkip || !hasAgentJudgeData(c.judge) {
			continue
		}
		if out == nil {
			out = &evalspb.AgentEvalResults_JudgeInfo{
				Model:           c.judge.GetModel(),
				JudgeCallCount:  c.judge.GetJudgeCallCount(),
				JudgeErrorCount: c.judge.GetJudgeErrorCount(),
			}
			if c.judge.ModelVersion != nil {
				version := c.judge.GetModelVersion()
				out.ModelVersion = &version
			}
			continue
		}
		out.JudgeCallCount += c.judge.GetJudgeCallCount()
		out.JudgeErrorCount += c.judge.GetJudgeErrorCount()
	}
	return out
}

func hasAgentJudgeData(j *evalspb.AgentEvalResults_JudgeInfo) bool {
	return j != nil &&
		(j.GetModel() != "" ||
			j.ModelVersion != nil ||
			j.GetJudgeCallCount() != 0 ||
			j.GetJudgeErrorCount() != 0)
}

func sameAgentJudgeDeclaration(a, b *evalspb.AgentEvalResults_JudgeInfo) bool {
	return a.GetModel() == b.GetModel() && a.GetModelVersion() == b.GetModelVersion()
}

func runAgentEvalCase(ctx context.Context, suiteName string, fc registeredCase) executedCase {
	fn, _ := fc.fn.(AgentEvalCaseFunc)
	r := newAgentEvalResult()
	if fn != nil {
		fn(ctx, r)
	}
	_ = suiteName
	return agentEvalOutcome(fc.name, r)
}

func agentEvalOutcome(caseName string, r *AgentEvalResult) executedCase {
	validations := validationsFromValidator(r.Validator())
	for _, err := range r.failures {
		validations = append(validations, &evalspb.Validation{
			Id:      caseValidationID,
			Status:  evalspb.Status_FAILED,
			Message: err.Error(),
		})
	}

	status := evalspb.Status_NOT_EVALUATED
	if r.sessionID != "" || len(r.metrics) > 0 || len(validations) > 0 || hasAgentJudgeData(r.judge) {
		status = evalspb.Status_PASSED
	}
	for _, metric := range r.metrics {
		if metric.GetStatus() == evalspb.Status_FAILED {
			status = evalspb.Status_FAILED
			break
		}
	}
	if status != evalspb.Status_FAILED {
		for _, v := range validations {
			if v.GetStatus() == evalspb.Status_FAILED {
				status = evalspb.Status_FAILED
				break
			}
		}
	}

	return executedCase{
		name:        caseName,
		status:      status,
		validations: validations,
		sessionID:   r.sessionID,
		metrics:     r.metrics,
		judge:       r.judge,
	}
}

func runLoadCase(ctx context.Context, suiteName string, fc registeredCase) executedCase {
	fn, _ := fc.fn.(LoadCaseFunc)
	r := newLoadResult()
	if fn != nil {
		fn(ctx, r)
	}
	_ = suiteName
	return loadOutcome(fc.name, r)
}

func loadOutcome(caseName string, r *LoadResult) executedCase {
	validations := validationsFromValidator(r.Validator())
	for _, err := range r.failures {
		validations = append(validations, &evalspb.Validation{
			Id:      caseValidationID,
			Status:  evalspb.Status_FAILED,
			Message: err.Error(),
		})
	}

	status := evalspb.Status_NOT_EVALUATED
	if r.summary != nil ||
		len(r.checks) > 0 ||
		len(r.tags) > 0 ||
		len(r.cloudRun) > 0 ||
		len(r.spanner) > 0 ||
		len(r.infraChecks) > 0 ||
		len(validations) > 0 {
		status = evalspb.Status_PASSED
	}
	for _, check := range r.checks {
		if check.GetStatus() == evalspb.Status_FAILED {
			status = evalspb.Status_FAILED
			break
		}
	}
	if status != evalspb.Status_FAILED {
		for _, check := range r.infraChecks {
			if check.GetStatus() == evalspb.Status_FAILED {
				status = evalspb.Status_FAILED
				break
			}
		}
	}
	if status != evalspb.Status_FAILED {
		for _, v := range validations {
			if v.GetStatus() == evalspb.Status_FAILED {
				status = evalspb.Status_FAILED
				break
			}
		}
	}

	return executedCase{
		name:        caseName,
		status:      status,
		validations: validations,
		summary:     r.summary,
		loadChecks:  r.checks,
		tags:        r.tags,
		cloudRun:    r.cloudRun,
		spanner:     r.spanner,
		infraChecks: r.infraChecks,
	}
}

func runInfraObservationCase(ctx context.Context, suiteName string, fc registeredCase) executedCase {
	fn, _ := fc.fn.(InfraObservationCaseFunc)
	r := newInfraObservationResult()
	if fn != nil {
		fn(ctx, r)
	}
	_ = suiteName
	return infraObservationOutcome(fc.name, r)
}

func infraObservationOutcome(caseName string, r *InfraObservationResult) executedCase {
	validations := validationsFromValidator(r.Validator())
	for _, err := range r.failures {
		validations = append(validations, &evalspb.Validation{
			Id:      caseValidationID,
			Status:  evalspb.Status_FAILED,
			Message: err.Error(),
		})
	}

	status := evalspb.Status_NOT_EVALUATED
	if r.windowSet ||
		len(r.cloudRun) > 0 ||
		len(r.spanner) > 0 ||
		len(r.infraChecks) > 0 ||
		len(validations) > 0 {
		status = evalspb.Status_PASSED
	}
	for _, check := range r.infraChecks {
		if check.GetStatus() == evalspb.Status_FAILED {
			status = evalspb.Status_FAILED
			break
		}
	}
	if status != evalspb.Status_FAILED && infraSnapshotsUnavailable(r.cloudRun, r.spanner) {
		status = evalspb.Status_FAILED
	}
	if status != evalspb.Status_FAILED {
		for _, v := range validations {
			if v.GetStatus() == evalspb.Status_FAILED {
				status = evalspb.Status_FAILED
				break
			}
		}
	}

	return executedCase{
		name:        caseName,
		status:      status,
		validations: validations,
		cloudRun:    r.cloudRun,
		spanner:     r.spanner,
		infraChecks: r.infraChecks,
		lookback:    r.lookback,
		windowStart: r.windowStart,
		windowEnd:   r.windowEnd,
		windowSet:   r.windowSet,
	}
}

func infraSnapshotsUnavailable(cloudRun []*evalspb.CloudRunTargetSnapshot, spanner []*evalspb.SpannerTargetSnapshot) bool {
	for _, snapshot := range cloudRun {
		if snapshot.GetFetchStatus() == evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE {
			return true
		}
	}
	for _, snapshot := range spanner {
		if snapshot.GetFetchStatus() == evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE {
			return true
		}
	}
	return false
}

func (s *suiteCore) materializeRun(cfg runConfig, cases []executedCase, start, end time.Time) *evalspb.Run {
	if s.branch == branchAgentEval {
		reconcileAgentJudge(cases)
	}
	runID := newUUID()
	run := &evalspb.Run{
		Name:            "runs/" + runID,
		Type:            s.runType(),
		Status:          rollupExecutedCases(cases),
		StartTime:       timestamppb.New(start),
		EndTime:         timestamppb.New(end),
		CreateTime:      timestamppb.Now(),
		Operation:       cfg.operation,
		GoogleProjectId: resolveGoogleProjectID(cfg),
	}
	if cfg.batchID != "" {
		run.BatchId = new(cfg.batchID)
	}
	s.attachBranchData(run, cases)
	return run
}

func rollupExecutedCases(cases []executedCase) evalspb.Status {
	if len(cases) == 0 {
		return evalspb.Status_PASSED
	}
	status := evalspb.Status_PASSED
	for _, c := range cases {
		switch c.status {
		case evalspb.Status_FAILED:
			return evalspb.Status_FAILED
		case evalspb.Status_NOT_EVALUATED:
			status = evalspb.Status_NOT_EVALUATED
		}
	}
	return status
}

func (s *suiteCore) attachBranchData(run *evalspb.Run, cases []executedCase) {
	switch s.branch {
	case branchIntegration:
		protoCases := make([]*evalspb.IntegrationTestResults_Case, len(cases))
		for i, c := range cases {
			protoCases[i] = &evalspb.IntegrationTestResults_Case{
				Id:       qualifiedCaseID(s.name, c.name),
				Status:   c.status,
				Checks:   c.checks,
				Duration: durationpb.New(c.duration),
			}
		}
		run.Data = &evalspb.Run_IntegrationTest{IntegrationTest: &evalspb.IntegrationTestResults{Cases: protoCases}}
	case branchAgentEval:
		protoCases := make([]*evalspb.AgentEvalResults_Case, len(cases))
		for i, c := range cases {
			protoCases[i] = &evalspb.AgentEvalResults_Case{
				Id:          qualifiedCaseID(s.name, c.name),
				Status:      c.status,
				Duration:    durationpb.New(c.duration),
				SessionId:   c.sessionID,
				Metrics:     c.metrics,
				Validations: c.validations,
			}
		}
		run.Data = &evalspb.Run_AgentEval{AgentEval: &evalspb.AgentEvalResults{
			Cases: protoCases,
			Judge: agentJudge(cases),
		}}
	case branchLoad:
		protoCases := make([]*evalspb.LoadTestResults_Case, len(cases))
		for i, c := range cases {
			protoCases[i] = &evalspb.LoadTestResults_Case{
				Id:          qualifiedCaseID(s.name, c.name),
				Status:      c.status,
				Summary:     c.summary,
				Checks:      c.loadChecks,
				Tags:        c.tags,
				CloudRun:    c.cloudRun,
				Spanner:     c.spanner,
				InfraChecks: c.infraChecks,
				Validations: c.validations,
			}
		}
		run.Data = &evalspb.Run_LoadTest{LoadTest: &evalspb.LoadTestResults{Cases: protoCases}}
	case branchInfraObservation:
		protoCases := make([]*evalspb.InfraObservationResults_Case, len(cases))
		for i, c := range cases {
			pc := &evalspb.InfraObservationResults_Case{
				Id:          qualifiedCaseID(s.name, c.name),
				Status:      c.status,
				CloudRun:    c.cloudRun,
				Spanner:     c.spanner,
				InfraChecks: c.infraChecks,
				Validations: c.validations,
			}
			if c.windowSet {
				pc.Lookback = durationpb.New(c.lookback)
				pc.WindowStart = timestamppb.New(c.windowStart)
				pc.WindowEnd = timestamppb.New(c.windowEnd)
			}
			protoCases[i] = pc
		}
		run.Data = &evalspb.Run_InfraObservation{InfraObservation: &evalspb.InfraObservationResults{Cases: protoCases}}
	}
}
