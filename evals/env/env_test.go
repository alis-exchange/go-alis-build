package env

import (
	"context"
	"errors"
	"testing"
)

func TestRegister_nilOption(t *testing.T) {
	t.Parallel()

	if err := Register("nil-opt-"+t.Name(), nil); !errors.Is(err, ErrNilOption{}) {
		t.Fatalf("Register() error = %v, want ErrNilOption", err)
	}
}

func TestRegister_andGet(t *testing.T) {
	t.Parallel()

	if err := Register("test-env-"+t.Name(),
		WithSetup(func(context.Context) error { return nil }),
	); err != nil {
		t.Fatalf("Register: %v", err)
	}

	e := Get("test-env-" + t.Name())
	if e == nil {
		t.Fatal("expected registered environment")
	}
	if e.Name() != "test-env-"+t.Name() {
		t.Fatalf("name = %q", e.Name())
	}
	if e.Setup() == nil {
		t.Fatal("expected setup hook")
	}
}

func TestRegister_duplicateReturnsError(t *testing.T) {
	t.Parallel()

	name := "dup-" + t.Name()
	if err := Register(name); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := Register(name); err == nil {
		t.Fatal("expected error on duplicate registration")
	}
}

func TestMustRegister_duplicatePanics(t *testing.T) {
	t.Parallel()

	name := "must-dup-" + t.Name()
	MustRegister(name)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	MustRegister(name)
}

func TestGet_unknownReturnsNil(t *testing.T) {
	t.Parallel()

	if e := Get("does-not-exist-" + t.Name()); e != nil {
		t.Fatalf("Get = %v, want nil", e)
	}
}

func TestRegistry_isolatedFromDefault(t *testing.T) {
	t.Parallel()

	regA := New()
	regB := New()
	name := "isolated-" + t.Name()
	if err := regA.Register(name); err != nil {
		t.Fatalf("regA Register: %v", err)
	}
	if regB.Get(name) != nil {
		t.Fatal("regB should not see regA environments")
	}
	if regA.Get(name) == nil {
		t.Fatal("regA should see its own environment")
	}
}

func TestDefaultRegistry_returnsProcessWideRegistry(t *testing.T) {
	t.Parallel()

	resetDefaultRegistryForTest(t)
	name := "default-" + t.Name()
	if err := Register(name); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if DefaultRegistry().Get(name) == nil {
		t.Fatal("DefaultRegistry should see registered environment")
	}
}

func TestEnvironment_nilSafeAccessors(t *testing.T) {
	t.Parallel()

	var e *Environment
	if e.Name() != "" {
		t.Fatal("nil environment name should be empty")
	}
	if e.Setup() != nil || e.Teardown() != nil {
		t.Fatal("nil environment hooks should be nil")
	}
}
