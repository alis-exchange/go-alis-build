package evals

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrSuiteSealed is recorded when AddCase races after the first Run seals definitions.
var ErrSuiteSealed = errors.New("evals: suite sealed")

// ErrEmptySuiteName is returned when a suite constructor receives an empty name.
type ErrEmptySuiteName struct{}

func (ErrEmptySuiteName) Error() string { return "evals: empty suite name" }

func (e ErrEmptySuiteName) Is(target error) bool {
	var err ErrEmptySuiteName
	return errors.As(target, &err)
}

func (e ErrEmptySuiteName) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidCaseName is returned when a case name is empty or contains '.'.
type ErrInvalidCaseName struct {
	Case string
}

func (e ErrInvalidCaseName) Error() string {
	if e.Case == "" {
		return "evals: empty case name"
	}
	return fmt.Sprintf("evals: invalid case name %q", e.Case)
}

func (e ErrInvalidCaseName) Is(target error) bool {
	var err ErrInvalidCaseName
	return errors.As(target, &err)
}

func (e ErrInvalidCaseName) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrDuplicateCase is returned when the same case name is registered twice.
type ErrDuplicateCase struct {
	Case string
}

func (e ErrDuplicateCase) Error() string {
	return fmt.Sprintf("evals: duplicate case name %q", e.Case)
}

func (e ErrDuplicateCase) Is(target error) bool {
	var err ErrDuplicateCase
	return errors.As(target, &err)
}

func (e ErrDuplicateCase) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidConcurrency is returned when WithMaxConcurrency is non-positive.
type ErrInvalidConcurrency struct {
	Value int
}

func (e ErrInvalidConcurrency) Error() string {
	return fmt.Sprintf("evals: max concurrency must be positive, got %d", e.Value)
}

func (e ErrInvalidConcurrency) Is(target error) bool {
	var err ErrInvalidConcurrency
	return errors.As(target, &err)
}

func (e ErrInvalidConcurrency) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNilReporter is returned when WithReporter is explicitly nil.
type ErrNilReporter struct{}

func (ErrNilReporter) Error() string { return "evals: nil reporter" }

func (e ErrNilReporter) Is(target error) bool {
	var err ErrNilReporter
	return errors.As(target, &err)
}

func (e ErrNilReporter) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNilCaseFunc is returned when a case is registered with a nil function.
type ErrNilCaseFunc struct {
	Case string
}

func (e ErrNilCaseFunc) Error() string {
	return fmt.Sprintf("case %q: nil func", e.Case)
}

func (e ErrNilCaseFunc) Is(target error) bool {
	var err ErrNilCaseFunc
	return errors.As(target, &err)
}

func (e ErrNilCaseFunc) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNilOption is returned when a nil run option is supplied.
type ErrNilOption struct{}

func (ErrNilOption) Error() string { return "evals: nil option" }

func (e ErrNilOption) Is(target error) bool {
	var err ErrNilOption
	return errors.As(target, &err)
}

func (e ErrNilOption) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNilStream is returned when a stream opener returns no stream and no error.
type ErrNilStream struct{}

func (ErrNilStream) Error() string { return "evals: stream opener returned nil stream" }

func (e ErrNilStream) Is(target error) bool {
	var err ErrNilStream
	return errors.As(target, &err)
}

func (e ErrNilStream) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrNilStreamMessage is returned when Recv returns no message and no error.
type ErrNilStreamMessage struct{}

func (ErrNilStreamMessage) Error() string {
	return "evals: server stream Recv returned nil message"
}

func (e ErrNilStreamMessage) Is(target error) bool {
	var err ErrNilStreamMessage
	return errors.As(target, &err)
}

func (e ErrNilStreamMessage) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ConfigErrors aggregates deferred suite configuration defects.
type ConfigErrors struct {
	errs []error
}

func (c *ConfigErrors) Error() string {
	if c == nil || len(c.errs) == 0 {
		return "evals: configuration errors"
	}
	parts := make([]string, len(c.errs))
	for i, err := range c.errs {
		parts[i] = err.Error()
	}
	return strings.Join(parts, "; ")
}

func (c *ConfigErrors) Is(target error) bool {
	var err *ConfigErrors
	return errors.As(target, &err)
}

func (c *ConfigErrors) Unwrap() []error {
	if c == nil {
		return nil
	}
	return append([]error(nil), c.errs...)
}

func (c *ConfigErrors) add(err error) {
	if err == nil {
		return
	}
	c.errs = append(c.errs, err)
}

func joinConfigErrors(errs ...error) error {
	var agg ConfigErrors
	for _, err := range errs {
		agg.add(err)
	}
	if len(agg.errs) == 0 {
		return nil
	}
	return &agg
}
