package evals

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNilCaseFunc is returned when a case is registered with a nil case
// function on a [Suite].
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

// ErrNilTarget is returned when a load case is registered with a nil
// target on a [LoadSuite].
type ErrNilTarget struct {
	Case string
}

func (e ErrNilTarget) Error() string {
	return fmt.Sprintf("load case %q: nil target", e.Case)
}

func (e ErrNilTarget) Is(target error) bool {
	var err ErrNilTarget
	return errors.As(target, &err)
}

func (e ErrNilTarget) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrEmptySLOs is returned when a load case is registered with no SLOs and
// without the [NoSLOs] escape hatch.
type ErrEmptySLOs struct {
	Case string
}

func (e ErrEmptySLOs) Error() string {
	return fmt.Sprintf("load case %q: at least one SLO required (use evals.NoSLOs() for measure-only cases)", e.Case)
}

func (e ErrEmptySLOs) Is(target error) bool {
	var err ErrEmptySLOs
	return errors.As(target, &err)
}

func (e ErrEmptySLOs) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrDuplicateSLOID is returned when a load case declares the same SLO id twice.
type ErrDuplicateSLOID struct {
	Case string
	ID   string
}

func (e ErrDuplicateSLOID) Error() string {
	return fmt.Sprintf("load case %q: duplicate SLO id %q", e.Case, e.ID)
}

func (e ErrDuplicateSLOID) Is(target error) bool {
	var err ErrDuplicateSLOID
	return errors.As(target, &err)
}

func (e ErrDuplicateSLOID) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrDualLoadCaseData is returned when both static data and a data provider
// are configured on the same load case.
type ErrDualLoadCaseData struct {
	Case string
}

func (e ErrDualLoadCaseData) Error() string {
	return fmt.Sprintf("load case %q: WithLoadCaseData and WithLoadCaseDataProvider are mutually exclusive", e.Case)
}

func (e ErrDualLoadCaseData) Is(target error) bool {
	var err ErrDualLoadCaseData
	return errors.As(target, &err)
}

func (e ErrDualLoadCaseData) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidSLO is returned when a load case declares an out-of-range SLO.
type ErrInvalidSLO struct {
	Case   string
	ID     string
	Reason string
}

func (e ErrInvalidSLO) Error() string {
	if e.ID == "" {
		return fmt.Sprintf("load case %q: invalid SLO: %s", e.Case, e.Reason)
	}
	return fmt.Sprintf("load case %q: SLO %q: %s", e.Case, e.ID, e.Reason)
}

func (e ErrInvalidSLO) Is(target error) bool {
	var err ErrInvalidSLO
	return errors.As(target, &err)
}

func (e ErrInvalidSLO) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNilProvider is returned when [RegisterAgent] is called with a nil
// provider.
type ErrNilProvider struct{}

func (e ErrNilProvider) Error() string { return "agent eval provider is nil" }

func (e ErrNilProvider) Is(target error) bool {
	var err ErrNilProvider
	return errors.As(target, &err)
}

func (e ErrNilProvider) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrUnknownSuiteKind is returned when a [Suite]'s kind field does not
// match any known [SuiteKind]. This indicates an internal invariant
// violation and normally cannot be produced through the public API,
// which sets `kind` via the exported constructors.
type ErrUnknownSuiteKind struct {
	Kind SuiteKind
}

func (e ErrUnknownSuiteKind) Error() string {
	return fmt.Sprintf("unknown suite kind %d", int(e.Kind))
}

func (e ErrUnknownSuiteKind) Is(target error) bool {
	var err ErrUnknownSuiteKind
	return errors.As(target, &err)
}

func (e ErrUnknownSuiteKind) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrWrongSuiteKind is returned when a suite is published with the wrong
// registration function (for example an eval suite passed to
// [RegisterIntegration]).
type ErrWrongSuiteKind struct {
	Suite string
	Want  SuiteKind
	Got   SuiteKind
}

func (e ErrWrongSuiteKind) Error() string {
	return fmt.Sprintf("suite %q is %s; expected %s", e.Suite, e.Got, e.Want)
}

func (e ErrWrongSuiteKind) Is(target error) bool {
	var err ErrWrongSuiteKind
	return errors.As(target, &err)
}

func (e ErrWrongSuiteKind) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNilStream is returned when openFn succeeds but returns a nil stream
// with a nil error.
type ErrNilStream struct{}

func (e ErrNilStream) Error() string {
	return "evals: openFn returned nil stream"
}

func (e ErrNilStream) Is(target error) bool {
	var err ErrNilStream
	return errors.As(target, &err)
}

func (e ErrNilStream) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrNilStreamMessage is returned when Recv returns a nil message with a
// nil error on a server stream.
type ErrNilStreamMessage struct{}

func (e ErrNilStreamMessage) Error() string {
	return "evals: server stream Recv returned nil message"
}

func (e ErrNilStreamMessage) Is(target error) bool {
	var err ErrNilStreamMessage
	return errors.As(target, &err)
}

func (e ErrNilStreamMessage) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}
