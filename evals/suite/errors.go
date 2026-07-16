package suite

import (
	"errors"
	"fmt"
	"time"

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
	// err carries the parse failure detail when Error() should surface it directly.
	err error
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

// ErrInfraTargetsEmpty is returned when a With*Targets option receives no targets.
type ErrInfraTargetsEmpty struct {
	Kind string
}

func (e ErrInfraTargetsEmpty) Error() string {
	return fmt.Sprintf("%s targets: at least one target required", e.Kind)
}

func (e ErrInfraTargetsEmpty) Is(target error) bool {
	var err ErrInfraTargetsEmpty
	return errors.As(target, &err)
}

func (e ErrInfraTargetsEmpty) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInfraDuplicateID is returned when infra target IDs collide across kinds.
type ErrInfraDuplicateID struct {
	ID string
}

func (e ErrInfraDuplicateID) Error() string {
	return fmt.Sprintf("duplicate infra target id %q", e.ID)
}

func (e ErrInfraDuplicateID) Is(target error) bool {
	var err ErrInfraDuplicateID
	return errors.As(target, &err)
}

func (e ErrInfraDuplicateID) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInfraCloudRunEntry is returned when Cloud Run targets lack exactly one ENTRY.
type ErrInfraCloudRunEntry struct {
	EntryCount int
}

func (e ErrInfraCloudRunEntry) Error() string {
	return fmt.Sprintf("cloud run targets: want exactly one ENTRY, got %d", e.EntryCount)
}

func (e ErrInfraCloudRunEntry) Is(target error) bool {
	var err ErrInfraCloudRunEntry
	return errors.As(target, &err)
}

func (e ErrInfraCloudRunEntry) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInfraSpannerDatabase is returned when a Spanner target has an empty database.
type ErrInfraSpannerDatabase struct {
	ID string
}

func (e ErrInfraSpannerDatabase) Error() string {
	return fmt.Sprintf("spanner target %q: database is required", e.ID)
}

func (e ErrInfraSpannerDatabase) Is(target error) bool {
	var err ErrInfraSpannerDatabase
	return errors.As(target, &err)
}

func (e ErrInfraSpannerDatabase) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInfraCloudRunTargetIncomplete is returned when a Cloud Run target lacks
// required identity fields.
type ErrInfraCloudRunTargetIncomplete struct {
	ID string
}

func (e ErrInfraCloudRunTargetIncomplete) Error() string {
	return fmt.Sprintf("cloud run target %q: id, project, region, and service_name are required", e.ID)
}

func (e ErrInfraCloudRunTargetIncomplete) Is(target error) bool {
	var err ErrInfraCloudRunTargetIncomplete
	return errors.As(target, &err)
}

func (e ErrInfraCloudRunTargetIncomplete) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInfraSpannerTargetIncomplete is returned when a Spanner target lacks
// required identity fields.
type ErrInfraSpannerTargetIncomplete struct {
	ID string
}

func (e ErrInfraSpannerTargetIncomplete) Error() string {
	return fmt.Sprintf("spanner target %q: id, project, instance, and location are required", e.ID)
}

func (e ErrInfraSpannerTargetIncomplete) Is(target error) bool {
	var err ErrInfraSpannerTargetIncomplete
	return errors.As(target, &err)
}

func (e ErrInfraSpannerTargetIncomplete) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInfraObserveLookbackUnset is returned when no lookback is configured.
type ErrInfraObserveLookbackUnset struct{}

func (e ErrInfraObserveLookbackUnset) Error() string {
	return "infra observation lookback unset: set WithLookback on the suite, WithObserveCaseLookback on a case, or pass lookback on RunInfraObservationRequest"
}

func (e ErrInfraObserveLookbackUnset) Is(target error) bool {
	var err ErrInfraObserveLookbackUnset
	return errors.As(target, &err)
}

func (e ErrInfraObserveLookbackUnset) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidLookback is returned when a lookback duration is not positive.
type ErrInvalidLookback struct {
	Value time.Duration
}

func (e ErrInvalidLookback) Error() string {
	return fmt.Sprintf("lookback must be positive, got %v", e.Value)
}

func (e ErrInvalidLookback) Is(target error) bool {
	var err ErrInvalidLookback
	return errors.As(target, &err)
}

func (e ErrInvalidLookback) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInfraObserveNoTargets is returned when an infra-observe suite declares
// no Cloud Run or Spanner targets.
type ErrInfraObserveNoTargets struct{}

func (e ErrInfraObserveNoTargets) Error() string {
	return "infra observation suite: at least one Cloud Run or Spanner target is required"
}

func (e ErrInfraObserveNoTargets) Is(target error) bool {
	var err ErrInfraObserveNoTargets
	return errors.As(target, &err)
}

func (e ErrInfraObserveNoTargets) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNilInfraObserveCase is returned when AddCase receives a nil case.
type ErrNilInfraObserveCase struct{}

func (e ErrNilInfraObserveCase) Error() string { return "nil infra observe case" }

func (e ErrNilInfraObserveCase) Is(target error) bool {
	var err ErrNilInfraObserveCase
	return errors.As(target, &err)
}

func (e ErrNilInfraObserveCase) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}
