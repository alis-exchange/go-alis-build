package loadgen

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrInvalidProfile is returned when a [Profile] fails validation.
type ErrInvalidProfile struct {
	Field string
	Want  string
	Got   string
}

func (e ErrInvalidProfile) Error() string {
	if e.Field == "" {
		return "loadgen: invalid profile"
	}
	if e.Want != "" || e.Got != "" {
		return fmt.Sprintf("loadgen: invalid profile field %s: got %s, want %s", e.Field, e.Got, e.Want)
	}
	return fmt.Sprintf("loadgen: invalid profile field %s", e.Field)
}

func (e ErrInvalidProfile) Is(target error) bool {
	var err ErrInvalidProfile
	if !errors.As(target, &err) {
		return false
	}
	if e.Field == "" || err.Field == "" {
		return true
	}
	return e.Field == err.Field
}

func (e ErrInvalidProfile) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNilTarget is returned when Generator.Run is called with a nil target.
type ErrNilTarget struct{}

func (e ErrNilTarget) Error() string { return "loadgen: nil target" }

func (e ErrNilTarget) Is(target error) bool {
	var err ErrNilTarget
	return errors.As(target, &err)
}

func (e ErrNilTarget) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrTargetPanic is returned when a load target panics during execution.
type ErrTargetPanic struct {
	Value any
	Stack string
}

func (e ErrTargetPanic) Error() string {
	return fmt.Sprintf("loadgen: target panic: %v\n%s", e.Value, e.Stack)
}

func (e ErrTargetPanic) Is(target error) bool {
	var err ErrTargetPanic
	return errors.As(target, &err)
}

func (e ErrTargetPanic) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}
