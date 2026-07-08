package registry

import (
	"context"
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/suite"
)

type stubTestCase struct {
	name   string
	result *execution.CaseResult
}

func (c stubTestCase) Name() string { return c.name }

func (c stubTestCase) Run(context.Context) *execution.CaseResult {
	return c.result
}

type stubEvalCase struct {
	name   string
	result *execution.CaseResult
}

func (c stubEvalCase) Name() string { return c.name }

func (c stubEvalCase) Run(context.Context) *execution.CaseResult {
	return c.result
}

func mustTestSuite(t *testing.T, name string, cases ...suite.TestCase) *suite.TestSuite {
	t.Helper()
	s, err := suite.NewTestSuite(name)
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCases(cases...); err != nil {
		t.Fatalf("AddCases: %v", err)
	}
	return s
}

func TestRegistry_SelectTestRuns_wholeSuite(t *testing.T) {
	t.Parallel()

	reg := New()
	s := mustTestSuite(t, "files-v2",
		stubTestCase{name: "a"},
		stubTestCase{name: "b"},
	)
	reg.RegisterIntegrationSuite(s)

	runs, err := reg.SelectTestRuns(evalspb.Run_INTEGRATION_TEST, []string{"files-v2"})
	if err != nil {
		t.Fatalf("SelectTestRuns: %v", err)
	}
	if len(runs) != 1 || len(runs[0].Cases) != 2 {
		t.Fatalf("runs = %#v, want one suite with two cases", runs)
	}
}

func TestRegistry_SelectTestRuns_casePath(t *testing.T) {
	t.Parallel()

	reg := New()
	reg.RegisterIntegrationSuite(mustTestSuite(t, "files-v2",
		stubTestCase{name: "upload"},
		stubTestCase{name: "delete"},
	))

	runs, err := reg.SelectTestRuns(evalspb.Run_INTEGRATION_TEST, []string{"files-v2.upload"})
	if err != nil {
		t.Fatalf("SelectTestRuns: %v", err)
	}
	if len(runs) != 1 || len(runs[0].Cases) != 1 || runs[0].Cases[0].Name() != "files-v2.upload" {
		t.Fatalf("runs = %#v", runs)
	}
}

func TestRegistry_SelectTestRuns_partialCasesShareSuite(t *testing.T) {
	t.Parallel()

	reg := New()
	reg.RegisterIntegrationSuite(mustTestSuite(t, "files-v2",
		stubTestCase{name: "upload"},
		stubTestCase{name: "delete"},
		stubTestCase{name: "list"},
	))

	runs, err := reg.SelectTestRuns(evalspb.Run_INTEGRATION_TEST, []string{"files-v2.upload", "files-v2.delete"})
	if err != nil {
		t.Fatalf("SelectTestRuns: %v", err)
	}
	if len(runs) != 1 || len(runs[0].Cases) != 2 {
		t.Fatalf("runs = %#v, want one run with two cases", runs)
	}
}

func TestRegistry_SelectTestRuns_allWhenEmptyFilter(t *testing.T) {
	t.Parallel()

	reg := New()
	reg.RegisterIntegrationSuite(mustTestSuite(t, "a", stubTestCase{name: "one"}))
	reg.RegisterIntegrationSuite(mustTestSuite(t, "b", stubTestCase{name: "two"}))

	runs, err := reg.SelectTestRuns(evalspb.Run_INTEGRATION_TEST, nil)
	if err != nil {
		t.Fatalf("SelectTestRuns: %v", err)
	}
	if len(runs) != 2 || suite.TotalTestCases(runs) != 2 {
		t.Fatalf("runs = %#v", runs)
	}
}

func TestRegistry_SelectTestRuns_invalidFilter(t *testing.T) {
	t.Parallel()

	reg := New()
	reg.RegisterIntegrationSuite(mustTestSuite(t, "a", stubTestCase{name: "one"}))

	_, err := reg.SelectTestRuns(evalspb.Run_INTEGRATION_TEST, []string{"a.b.c"})
	if err == nil {
		t.Fatal("expected error for invalid filter path")
	}
}

func TestRegistry_SelectTestRuns_unknownSuite(t *testing.T) {
	t.Parallel()

	reg := New()
	reg.RegisterIntegrationSuite(mustTestSuite(t, "a", stubTestCase{name: "one"}))

	runs, err := reg.SelectTestRuns(evalspb.Run_INTEGRATION_TEST, []string{"missing"})
	if err != nil {
		t.Fatalf("SelectTestRuns: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("runs = %#v, want empty", runs)
	}
}

func TestRegistry_ValidateSelection(t *testing.T) {
	t.Parallel()

	reg := New()
	reg.RegisterIntegrationSuite(mustTestSuite(t, "files-v2",
		stubTestCase{name: "list-files"},
		stubTestCase{name: "get-root"},
	))

	if err := reg.ValidateSelection(evalspb.Run_INTEGRATION_TEST, nil); err != nil {
		t.Fatalf("empty filter: %v", err)
	}
	if err := reg.ValidateSelection(evalspb.Run_INTEGRATION_TEST, []string{"files-v2"}); err != nil {
		t.Fatalf("suite filter: %v", err)
	}
	if err := reg.ValidateSelection(evalspb.Run_INTEGRATION_TEST, []string{"missing"}); err == nil {
		t.Fatal("expected error for unknown suite")
	}
	if err := reg.ValidateSelection(evalspb.Run_INTEGRATION_TEST, []string{"files-v2.missing"}); err == nil {
		t.Fatal("expected error for unknown case")
	}
	if err := reg.ValidateSelection(evalspb.Run_INTEGRATION_TEST, []string{"a.b.c"}); err == nil {
		t.Fatal("expected error for malformed filter")
	}
}

func TestRegistry_ValidateSelection_agentEval(t *testing.T) {
	t.Parallel()

	s, err := suite.NewEvalSuite("core")
	if err != nil {
		t.Fatalf("NewEvalSuite: %v", err)
	}
	if err := s.AddCase(stubEvalCase{name: "placeholder-eval"}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	reg := New()
	reg.RegisterAgentEvalSuite(s)

	if err := reg.ValidateSelection(evalspb.Run_AGENT_EVAL, []string{"core"}); err != nil {
		t.Fatalf("suite filter: %v", err)
	}
	if err := reg.ValidateSelection(evalspb.Run_AGENT_EVAL, []string{"missing"}); err == nil {
		t.Fatal("expected error for unknown suite")
	}
}

func TestRegistry_ValidateSelection_agentEvalProviderOnly(t *testing.T) {
	t.Parallel()

	reg := New()
	reg.RegisterAgentEvalProvider(AgentEvalProviderFunc(func(context.Context, []string) ([]execution.SuiteResult, error) {
		return nil, nil
	}))

	if err := reg.ValidateSelection(evalspb.Run_AGENT_EVAL, nil); err != nil {
		t.Fatalf("empty filter with provider: %v", err)
	}
	if err := reg.ValidateSelection(evalspb.Run_AGENT_EVAL, []string{"unknown"}); err != nil {
		t.Fatalf("unknown filter with provider should defer validation: %v", err)
	}
}

func TestRegistry_AgentEvalProviders(t *testing.T) {
	t.Parallel()

	reg := New()
	if len(reg.AgentEvalProviders()) != 0 {
		t.Fatal("expected no providers initially")
	}
	p := AgentEvalProviderFunc(func(context.Context, []string) ([]execution.SuiteResult, error) {
		return nil, nil
	})
	reg.RegisterAgentEvalProvider(p)
	if len(reg.AgentEvalProviders()) != 1 {
		t.Fatalf("providers = %d", len(reg.AgentEvalProviders()))
	}
}

func TestRegistry_SelectEvalRuns(t *testing.T) {
	t.Parallel()

	s, err := suite.NewEvalSuite("evals")
	if err != nil {
		t.Fatalf("NewEvalSuite: %v", err)
	}
	if err := s.AddCase(stubEvalCase{name: "one"}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	reg := New()
	reg.RegisterAgentEvalSuite(s)

	runs, err := reg.SelectEvalRuns([]string{"evals.one"})
	if err != nil {
		t.Fatalf("SelectEvalRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].Cases[0].Name() != "evals.one" {
		t.Fatalf("runs = %#v", runs)
	}
}
