package env

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNotConfigured is returned when registry methods are called on a nil receiver.
type ErrNotConfigured struct{}

func (e ErrNotConfigured) Error() string { return "environment registry not configured" }

func (e ErrNotConfigured) Is(target error) bool {
	var err ErrNotConfigured
	return errors.As(target, &err)
}

func (e ErrNotConfigured) GRPCStatus() *status.Status {
	return status.New(codes.FailedPrecondition, e.Error())
}

// ErrNilOption is returned when Register is called with a nil option.
type ErrNilOption struct{}

func (e ErrNilOption) Error() string { return "nil option" }

func (e ErrNilOption) Is(target error) bool {
	var err ErrNilOption
	return errors.As(target, &err)
}

func (e ErrNilOption) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrDuplicateRegistration is returned when Register is called with a
// name that is already registered.
type ErrDuplicateRegistration struct {
	Name string
}

func (e ErrDuplicateRegistration) Error() string {
	return fmt.Sprintf("environment %q already registered", e.Name)
}

func (e ErrDuplicateRegistration) Is(target error) bool {
	var err ErrDuplicateRegistration
	return errors.As(target, &err)
}

func (e ErrDuplicateRegistration) GRPCStatus() *status.Status {
	return status.New(codes.AlreadyExists, e.Error())
}

// ErrNotRegistered is returned when a named environment has not been registered.
type ErrNotRegistered struct {
	Name string
}

func (e ErrNotRegistered) Error() string {
	return fmt.Sprintf("environment %q not registered", e.Name)
}

func (e ErrNotRegistered) Is(target error) bool {
	var err ErrNotRegistered
	return errors.As(target, &err)
}

func (e ErrNotRegistered) GRPCStatus() *status.Status {
	return status.New(codes.FailedPrecondition, e.Error())
}

// ErrSetupFailed is returned when environment setup fails.
type ErrSetupFailed struct {
	Name string
	Err  error
}

func NewSetupFailed(name string, err error) ErrSetupFailed {
	return ErrSetupFailed{Name: name, Err: err}
}

func (e ErrSetupFailed) Error() string {
	return fmt.Sprintf("environment %q setup: %v", e.Name, e.Err)
}

func (e ErrSetupFailed) Unwrap() error { return e.Err }

func (e ErrSetupFailed) Is(target error) bool {
	var err ErrSetupFailed
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrSetupFailed) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}
