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
