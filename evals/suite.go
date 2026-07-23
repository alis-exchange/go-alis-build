package evals

import (
	"context"
	"reflect"
	"strings"
	"sync"

	evalspb "go.alis.build/common/alis/evals/v1"
)

type suiteBranch int

const (
	branchIntegration suiteBranch = iota
	branchAgentEval
	branchLoad
	branchInfraObservation
)

type suiteCore struct {
	mu sync.Mutex

	name        string
	branch      suiteBranch
	sealed      bool
	cases       []registeredCase
	pendingErrs []error
}

type registeredCase struct {
	name string
	fn   any
}

func newSuiteCore(name string, branch suiteBranch) *suiteCore {
	s := &suiteCore{name: name, branch: branch}
	if strings.TrimSpace(name) == "" {
		s.pendingErrs = append(s.pendingErrs, ErrEmptySuiteName{})
	}
	return s
}

func (s *suiteCore) addCase(name string, fn any) *suiteCore {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sealed {
		s.pendingErrs = append(s.pendingErrs, ErrSuiteSealed)
		return s
	}
	if err := validateCaseName(name); err != nil {
		s.pendingErrs = append(s.pendingErrs, err)
		return s
	}
	if isNilCaseFunc(fn) {
		s.pendingErrs = append(s.pendingErrs, ErrNilCaseFunc{Case: name})
		return s
	}
	for _, c := range s.cases {
		if c.name == name {
			s.pendingErrs = append(s.pendingErrs, ErrDuplicateCase{Case: name})
			return s
		}
	}
	s.cases = append(s.cases, registeredCase{name: name, fn: fn})
	return s
}

func validateCaseName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrInvalidCaseName{}
	}
	if strings.Contains(name, ".") {
		return ErrInvalidCaseName{Case: name}
	}
	return nil
}

func isNilCaseFunc(fn any) bool {
	if fn == nil {
		return true
	}
	v := reflect.ValueOf(fn)
	return v.Kind() == reflect.Func && v.IsNil()
}

func (s *suiteCore) run(ctx context.Context, opts []RunOption, publish bool) (*evalspb.Run, error) {
	s.mu.Lock()
	if !s.sealed {
		s.sealed = true
	}
	s.mu.Unlock()

	if err := joinConfigErrors(append(s.snapshotPendingErrors(), applyRunOptionsErrors(opts)...)...); err != nil {
		return nil, err
	}
	cfg, err := applyRunOptions(defaultRunConfig(), opts)
	if err != nil {
		return nil, err
	}

	cases := append([]registeredCase(nil), s.cases...)
	start := now()
	executed := s.executeCases(ctx, cfg, cases)
	end := now()
	run := s.materializeRun(cfg, executed, start, end)
	if publish && cfg.reporter != nil {
		if err := cfg.reporter.ReportRun(ctx, run); err != nil {
			return run, err
		}
	}
	return run, nil
}

func (s *suiteCore) snapshotPendingErrors() []error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pendingErrs) == 0 {
		return nil
	}
	out := append([]error(nil), s.pendingErrs...)
	return out
}

func applyRunOptionsErrors(opts []RunOption) []error {
	var errs []error
	for _, opt := range opts {
		if opt == nil {
			errs = append(errs, ErrNilOption{})
			continue
		}
		cfg := defaultRunConfig()
		if err := opt.apply(&cfg); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (s *suiteCore) runType() evalspb.Run_Type {
	switch s.branch {
	case branchIntegration:
		return evalspb.Run_INTEGRATION_TEST
	case branchAgentEval:
		return evalspb.Run_AGENT_EVAL
	case branchLoad:
		return evalspb.Run_LOAD_TEST
	case branchInfraObservation:
		return evalspb.Run_INFRA_OBSERVATION
	default:
		return evalspb.Run_TYPE_UNSPECIFIED
	}
}
