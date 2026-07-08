package env

import (
	"context"
	"testing"
)

func TestRegister_andGet(t *testing.T) {
	t.Parallel()

	Register("test-env-"+t.Name(),
		WithSetup(func(context.Context) error { return nil }),
	)

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

func TestRegister_duplicatePanics(t *testing.T) {
	t.Parallel()

	name := "dup-" + t.Name()
	Register(name)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	Register(name)
}

func TestGet_unknownReturnsNil(t *testing.T) {
	t.Parallel()

	if e := Get("does-not-exist-" + t.Name()); e != nil {
		t.Fatalf("Get = %v, want nil", e)
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
