package suite

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNilSuite is returned when a suite method is called on a nil receiver.
type ErrNilSuite struct{}

func (e ErrNilSuite) Error() string { return "suite is nil" }

func (e ErrNilSuite) Is(target error) bool {
	var err ErrNilSuite
	return errors.As(target, &err)
}

func (e ErrNilSuite) GRPCStatus() *status.Status {
	return status.New(codes.FailedPrecondition, e.Error())
}

// ErrUnknownEnvironment is returned when WithEnvironment references an unregistered name.
type ErrUnknownEnvironment struct {
	Name string
}

func (e ErrUnknownEnvironment) Error() string {
	return fmt.Sprintf("unknown environment %q", e.Name)
}

func (e ErrUnknownEnvironment) Is(target error) bool {
	var err ErrUnknownEnvironment
	return errors.As(target, &err)
}

func (e ErrUnknownEnvironment) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidSuiteName is returned when a suite name is empty or contains '.'.
type ErrInvalidSuiteName struct {
	Name   string
	Reason string
}

func (e ErrInvalidSuiteName) Error() string {
	if e.Name == "" {
		return e.Reason
	}
	return fmt.Sprintf("suite name %q: %s", e.Name, e.Reason)
}

func (e ErrInvalidSuiteName) Is(target error) bool {
	var err ErrInvalidSuiteName
	return errors.As(target, &err)
}

func (e ErrInvalidSuiteName) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidCaseName is returned when a case short name contains '.'.
type ErrInvalidCaseName struct {
	Name string
}

func (e ErrInvalidCaseName) Error() string {
	return fmt.Sprintf("case name %q must not contain '.'; suite qualifies names", e.Name)
}

func (e ErrInvalidCaseName) Is(target error) bool {
	var err ErrInvalidCaseName
	return errors.As(target, &err)
}

func (e ErrInvalidCaseName) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrDuplicateCase is returned when the same short case name is registered twice.
type ErrDuplicateCase struct {
	Suite string
	Case  string
}

func (e ErrDuplicateCase) Error() string {
	return fmt.Sprintf("duplicate case %q in suite %q", e.Case, e.Suite)
}

func (e ErrDuplicateCase) Is(target error) bool {
	var err ErrDuplicateCase
	return errors.As(target, &err)
}

func (e ErrDuplicateCase) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidFilterPath is returned when a case filter string is malformed.
type ErrInvalidFilterPath struct {
	Path string
	err  error
}

func (e ErrInvalidFilterPath) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	if e.Path == "" {
		return "empty filter path"
	}
	return fmt.Sprintf("invalid filter path %q", e.Path)
}

func (e ErrInvalidFilterPath) Unwrap() error { return e.err }

func (e ErrInvalidFilterPath) Is(target error) bool {
	var err ErrInvalidFilterPath
	return errors.As(target, &err) || errors.Is(e.err, target)
}

func (e ErrInvalidFilterPath) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrLoadProfileUnspecifiedMode is returned when WithLoadProfileOverride
// is called with RunLoadTestRequest_MODE_UNSPECIFIED. Overrides must
// target a concrete mode.
type ErrLoadProfileUnspecifiedMode struct{}

func (e ErrLoadProfileUnspecifiedMode) Error() string {
	return "load profile override: mode is UNSPECIFIED"
}

func (e ErrLoadProfileUnspecifiedMode) Is(target error) bool {
	var err ErrLoadProfileUnspecifiedMode
	return errors.As(target, &err)
}

func (e ErrLoadProfileUnspecifiedMode) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNilCaseResult is returned when a case Run method returns nil.
type ErrNilCaseResult struct{}

func (e ErrNilCaseResult) Error() string { return "case returned nil result" }

func (e ErrNilCaseResult) Is(target error) bool {
	var err ErrNilCaseResult
	return errors.As(target, &err)
}

func (e ErrNilCaseResult) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}
