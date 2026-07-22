package registry

import (
	"errors"
	"sync"
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/env"
)

func TestRegistry_duplicateIntegrationSuiteName(t *testing.T) {
	t.Parallel()

	reg := New()
	s1 := mustTestSuite(t, "dup-suite", stubTestCase{name: "a"})
	s2 := mustTestSuite(t, "dup-suite", stubTestCase{name: "b"})
	if err := reg.RegisterIntegrationSuite(s1); err != nil {
		t.Fatalf("first RegisterIntegrationSuite: %v", err)
	}
	err := reg.RegisterIntegrationSuite(s2)
	if !errors.As(err, new(ErrDuplicateSuite)) {
		t.Fatalf("second RegisterIntegrationSuite() error = %v, want ErrDuplicateSuite", err)
	}
}

func TestRegistry_concurrentFreezeAndReads(t *testing.T) {
	t.Parallel()

	reg := New()
	if err := reg.RegisterIntegrationSuite(mustTestSuite(t, "suite-a", stubTestCase{name: "a"})); err != nil {
		t.Fatalf("RegisterIntegrationSuite: %v", err)
	}
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = reg.Freeze()
		}()
		go func() {
			defer wg.Done()
			_, _ = reg.SelectTestRuns(evalspb.Run_INTEGRATION_TEST, nil)
			_ = reg.Frozen()
		}()
	}
	wg.Wait()
	if !reg.Frozen() {
		t.Fatal("registry was not frozen")
	}
}

func TestRegistry_registerAfterFreeze(t *testing.T) {
	t.Parallel()

	reg := New()
	if err := reg.RegisterIntegrationSuite(mustTestSuite(t, "suite-a", stubTestCase{name: "a"})); err != nil {
		t.Fatalf("RegisterIntegrationSuite: %v", err)
	}
	if err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if !reg.Frozen() {
		t.Fatal("Frozen() = false, want true")
	}
	err := reg.RegisterIntegrationSuite(mustTestSuite(t, "suite-b", stubTestCase{name: "b"}))
	if !errors.Is(err, ErrRegistryFrozen{}) {
		t.Fatalf("RegisterIntegrationSuite after Freeze() error = %v, want ErrRegistryFrozen", err)
	}
	if err := reg.SetEnvRegistry(env.New()); !errors.Is(err, ErrRegistryFrozen{}) {
		t.Fatalf("SetEnvRegistry after Freeze() error = %v, want ErrRegistryFrozen", err)
	}
}

func TestRegistry_customRegistryIsolatedFromDefault(t *testing.T) {
	t.Parallel()

	regA := New()
	regB := New()
	if err := regA.RegisterIntegrationSuite(mustTestSuite(t, "only-a", stubTestCase{name: "a"})); err != nil {
		t.Fatalf("RegisterIntegrationSuite: %v", err)
	}
	runs, err := regB.SelectTestRuns(evalspb.Run_INTEGRATION_TEST, []string{"only-a"})
	if err != nil {
		t.Fatalf("SelectTestRuns: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("isolated registry returned %d runs, want 0", len(runs))
	}
	runs, err = regA.SelectTestRuns(evalspb.Run_INTEGRATION_TEST, []string{"only-a"})
	if err != nil {
		t.Fatalf("SelectTestRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("regA returned %d runs, want 1", len(runs))
	}
}

func TestRegistry_freezeIdempotent(t *testing.T) {
	t.Parallel()

	reg := New()
	if err := reg.Freeze(); err != nil {
		t.Fatalf("first Freeze: %v", err)
	}
	if err := reg.Freeze(); err != nil {
		t.Fatalf("second Freeze: %v", err)
	}
}
