package registry

import (
	"errors"
	"fmt"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNotConfigured is returned when registry methods are called on a nil receiver.
type ErrNotConfigured struct{}

func (e ErrNotConfigured) Error() string { return "registry not configured" }

func (e ErrNotConfigured) Is(target error) bool {
	var err ErrNotConfigured
	return errors.As(target, &err)
}

func (e ErrNotConfigured) GRPCStatus() *status.Status {
	return status.New(codes.FailedPrecondition, e.Error())
}

// ErrNoSuites is returned when no suites are registered for a run type.
type ErrNoSuites struct {
	Type evalspb.Run_Type
}

func (e ErrNoSuites) Error() string {
	return fmt.Sprintf("no test suites registered for type %v", e.Type)
}

func (e ErrNoSuites) Is(target error) bool {
	var err ErrNoSuites
	return errors.As(target, &err)
}

func (e ErrNoSuites) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNoEvalSuites is returned when no agent eval suites are registered.
type ErrNoEvalSuites struct{}

func (e ErrNoEvalSuites) Error() string { return "no agent eval suites registered" }

func (e ErrNoEvalSuites) Is(target error) bool {
	var err ErrNoEvalSuites
	return errors.As(target, &err)
}

func (e ErrNoEvalSuites) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNoLoadSuites is returned when no load-test suites are registered.
type ErrNoLoadSuites struct{}

func (e ErrNoLoadSuites) Error() string { return "no load-test suites registered" }

func (e ErrNoLoadSuites) Is(target error) bool {
	var err ErrNoLoadSuites
	return errors.As(target, &err)
}

func (e ErrNoLoadSuites) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNoInfraObserveSuites is returned when no infra observation suites are registered.
type ErrNoInfraObserveSuites struct{}

func (e ErrNoInfraObserveSuites) Error() string { return "no infra observation suites registered" }

func (e ErrNoInfraObserveSuites) Is(target error) bool {
	var err ErrNoInfraObserveSuites
	return errors.As(target, &err)
}

func (e ErrNoInfraObserveSuites) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrUnknownCase is returned when case_ids contains an unknown filter.
type ErrUnknownCase struct {
	Name string
}

func (e ErrUnknownCase) Error() string {
	return fmt.Sprintf("unknown case id %q", e.Name)
}

func (e ErrUnknownCase) Is(target error) bool {
	var err ErrUnknownCase
	return errors.As(target, &err)
}

func (e ErrUnknownCase) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrUnsupportedRunType is returned for unsupported Run.Type values.
type ErrUnsupportedRunType struct {
	Type evalspb.Run_Type
}

func (e ErrUnsupportedRunType) Error() string {
	return fmt.Sprintf("unsupported run type %v", e.Type)
}

func (e ErrUnsupportedRunType) Is(target error) bool {
	var err ErrUnsupportedRunType
	return errors.As(target, &err)
}

func (e ErrUnsupportedRunType) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrRegistryFrozen is returned when registration is attempted after Freeze.
type ErrRegistryFrozen struct{}

func (e ErrRegistryFrozen) Error() string { return "registry is frozen" }

func (e ErrRegistryFrozen) Is(target error) bool {
	var err ErrRegistryFrozen
	return errors.As(target, &err)
}

func (e ErrRegistryFrozen) GRPCStatus() *status.Status {
	return status.New(codes.FailedPrecondition, e.Error())
}

// ErrDuplicateSuite is returned when the same suite name is registered twice
// within one run kind.
type ErrDuplicateSuite struct {
	Kind string
	Name string
}

func (e ErrDuplicateSuite) Error() string {
	return fmt.Sprintf("duplicate %s suite %q", e.Kind, e.Name)
}

func (e ErrDuplicateSuite) Is(target error) bool {
	var err ErrDuplicateSuite
	return errors.As(target, &err)
}

func (e ErrDuplicateSuite) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrUnknownEnvironments is returned when Freeze finds suite environment
// names that are not registered in the environment registry.
type ErrUnknownEnvironments struct {
	Names []string
}

func (e ErrUnknownEnvironments) Error() string {
	return fmt.Sprintf("unknown environments: %v", e.Names)
}

func (e ErrUnknownEnvironments) Is(target error) bool {
	var err ErrUnknownEnvironments
	return errors.As(target, &err)
}

func (e ErrUnknownEnvironments) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}
