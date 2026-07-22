package registry

import (
	"errors"
	"testing"

	"go.alis.build/evals/env"
	"go.alis.build/evals/suite"
)

func TestRegistry_Freeze_unknownEnvironment(t *testing.T) {
	t.Parallel()

	reg := New()
	envReg := env.New()
	if err := reg.SetEnvRegistry(envReg); err != nil {
		t.Fatalf("SetEnvRegistry: %v", err)
	}

	missing := "missing-" + t.Name()
	s, err := suite.NewTestSuite("files-v2", suite.WithEnvironment(missing))
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCase(stubTestCase{name: "upload"}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	if err := reg.RegisterIntegrationSuite(s); err != nil {
		t.Fatalf("RegisterIntegrationSuite: %v", err)
	}

	err = reg.Freeze()
	var unknown ErrUnknownEnvironments
	if !errors.As(err, &unknown) {
		t.Fatalf("Freeze() error = %v, want ErrUnknownEnvironments", err)
	}
	if len(unknown.Names) != 1 || unknown.Names[0] != missing {
		t.Fatalf("unknown names = %v, want [%q]", unknown.Names, missing)
	}
}

func TestRegistry_Freeze_deferredEnvironmentResolution(t *testing.T) {
	t.Parallel()

	reg := New()
	envReg := env.New()
	if err := reg.SetEnvRegistry(envReg); err != nil {
		t.Fatalf("SetEnvRegistry: %v", err)
	}

	envName := "deferred-" + t.Name()
	s, err := suite.NewTestSuite("files-v2", suite.WithEnvironment(envName))
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCase(stubTestCase{name: "upload"}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	if err := reg.RegisterIntegrationSuite(s); err != nil {
		t.Fatalf("RegisterIntegrationSuite: %v", err)
	}
	if err := envReg.Register(envName); err != nil {
		t.Fatalf("env Register: %v", err)
	}
	if err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
}

func TestRegistry_Freeze_listsAllMissingEnvironments(t *testing.T) {
	t.Parallel()

	reg := New()
	envReg := env.New()
	if err := reg.SetEnvRegistry(envReg); err != nil {
		t.Fatalf("SetEnvRegistry: %v", err)
	}

	missingA := "missing-a-" + t.Name()
	missingB := "missing-b-" + t.Name()
	s, err := suite.NewTestSuite("files-v2", suite.WithEnvironment(missingA, missingB))
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCase(stubTestCase{name: "upload"}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	if err := reg.RegisterIntegrationSuite(s); err != nil {
		t.Fatalf("RegisterIntegrationSuite: %v", err)
	}

	err = reg.Freeze()
	var unknown ErrUnknownEnvironments
	if !errors.As(err, &unknown) {
		t.Fatalf("Freeze() error = %v, want ErrUnknownEnvironments", err)
	}
	if len(unknown.Names) != 2 {
		t.Fatalf("unknown names = %v, want both missing envs", unknown.Names)
	}
}
