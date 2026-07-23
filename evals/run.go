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
				out := s.runOneCase(ctx, fc)
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
			return cases
		case jobs <- i:
		}
	}
	close(jobs)

	wg.Wait()
	return cases
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
	var panicValue any
	if fn != nil {
		func() {
			defer func() {
				panicValue = recover()
			}()
			fn(ctx, v)
		}()
	}
	if panicValue != nil {
		return executedCase{
			status: evalspb.Status_FAILED,
			checks: []*evalspb.IntegrationTestResults_Case_Check{
				{
					Id:      panicCheckID,
					Status:  evalspb.Status_FAILED,
					Message: fmt.Sprintf("panic: %v", panicValue),
				},
			},
		}
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
	_ = qualifiedCaseID(suiteName, caseName)
	return executedCase{
		name:     caseName,
		status:   status,
		duration: dur,
		checks:   checks,
	}
}

func runAgentEvalCase(ctx context.Context, suiteName string, fc registeredCase) executedCase {
	fn, _ := fc.fn.(AgentEvalCaseFunc)
	r := &AgentEvalResult{}
	if fn != nil {
		fn(ctx, r)
	}
	_ = suiteName
	return executedCase{
		name:   fc.name,
		status: evalspb.Status_PASSED,
	}
}

func runLoadCase(ctx context.Context, suiteName string, fc registeredCase) executedCase {
	fn, _ := fc.fn.(LoadCaseFunc)
	r := &LoadResult{}
	if fn != nil {
		fn(ctx, r)
	}
	_ = suiteName
	return executedCase{
		name:   fc.name,
		status: evalspb.Status_PASSED,
	}
}

func runInfraObservationCase(ctx context.Context, suiteName string, fc registeredCase) executedCase {
	fn, _ := fc.fn.(InfraObservationCaseFunc)
	r := &InfraObservationResult{}
	if fn != nil {
		fn(ctx, r)
	}
	_ = suiteName
	return executedCase{
		name:   fc.name,
		status: evalspb.Status_PASSED,
	}
}

func (s *suiteCore) materializeRun(cfg runConfig, cases []executedCase, start, end time.Time) *evalspb.Run {
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
				Id:       qualifiedCaseID(s.name, c.name),
				Status:   c.status,
				Duration: durationpb.New(c.duration),
			}
		}
		run.Data = &evalspb.Run_AgentEval{AgentEval: &evalspb.AgentEvalResults{Cases: protoCases}}
	case branchLoad:
		protoCases := make([]*evalspb.LoadTestResults_Case, len(cases))
		for i, c := range cases {
			protoCases[i] = &evalspb.LoadTestResults_Case{
				Id:     qualifiedCaseID(s.name, c.name),
				Status: c.status,
			}
		}
		run.Data = &evalspb.Run_LoadTest{LoadTest: &evalspb.LoadTestResults{Cases: protoCases}}
	case branchInfraObservation:
		protoCases := make([]*evalspb.InfraObservationResults_Case, len(cases))
		for i, c := range cases {
			protoCases[i] = &evalspb.InfraObservationResults_Case{
				Id:     qualifiedCaseID(s.name, c.name),
				Status: c.status,
			}
		}
		run.Data = &evalspb.Run_InfraObservation{InfraObservation: &evalspb.InfraObservationResults{Cases: protoCases}}
	}
}
