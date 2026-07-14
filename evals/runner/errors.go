package runner

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNilRunner is returned when RunEvalSuites is called on a nil runner.
type ErrNilRunner struct{}

func (e ErrNilRunner) Error() string { return "runner is nil" }

func (e ErrNilRunner) Is(target error) bool {
	var err ErrNilRunner
	return errors.As(target, &err)
}

func (e ErrNilRunner) GRPCStatus() *status.Status {
	return status.New(codes.FailedPrecondition, e.Error())
}

// ErrNilResult is returned when a case Run method returns nil.
type ErrNilResult struct{}

func (e ErrNilResult) Error() string { return "case returned nil result" }

func (e ErrNilResult) Is(target error) bool {
	var err ErrNilResult
	return errors.As(target, &err)
}

func (e ErrNilResult) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrNilLoadProfileResolver is returned when RunLoadSuites is called with a
// nil load profile resolver.
type ErrNilLoadProfileResolver struct{}

func (e ErrNilLoadProfileResolver) Error() string {
	return "runner: nil load profile resolver"
}

func (e ErrNilLoadProfileResolver) Is(target error) bool {
	var err ErrNilLoadProfileResolver
	return errors.As(target, &err)
}

func (e ErrNilLoadProfileResolver) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrCasePanic is returned when a case panics during execution.
type ErrCasePanic struct {
	Value any
	Stack string
}

func (e ErrCasePanic) Error() string {
	return fmt.Sprintf("panic: %v\n%s", e.Value, e.Stack)
}

func (e ErrCasePanic) Is(target error) bool {
	var err ErrCasePanic
	return errors.As(target, &err)
}

func (e ErrCasePanic) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}
