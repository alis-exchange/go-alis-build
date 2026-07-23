package evals

import (
	"context"
	"errors"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/validation"
	"google.golang.org/protobuf/proto"
)

// InfraObservationCaseFunc runs one infra-observation case against a result builder.
type InfraObservationCaseFunc func(context.Context, *InfraObservationResult)

var (
	errInfraWindowAlreadySet    = errors.New("evals: infra observation window already set")
	errNilInfraSLOCheck         = errors.New("evals: nil infra observation SLO check")
	errNilInfraCloudRunSnapshot = errors.New("evals: nil infra observation Cloud Run snapshot")
	errNilInfraSpannerSnapshot  = errors.New("evals: nil infra observation Spanner snapshot")
)

// InfraObservationResult collects protobuf-native observation case data.
type InfraObservationResult struct {
	validator   *validation.Validator
	lookback    time.Duration
	windowStart time.Time
	windowEnd   time.Time
	windowSet   bool
	cloudRun    []*evalspb.CloudRunTargetSnapshot
	spanner     []*evalspb.SpannerTargetSnapshot
	infraChecks []*evalspb.InfraSloCheck
	failures    []error
}

func newInfraObservationResult() *InfraObservationResult {
	return &InfraObservationResult{validator: validation.NewValidator()}
}

// Validator returns the case-local validator used for general validation rules.
func (r *InfraObservationResult) Validator() *validation.Validator {
	if r.validator == nil {
		r.validator = validation.NewValidator()
	}
	return r.validator
}

// Fail records a case failure while preserving any data already added.
func (r *InfraObservationResult) Fail(err error) {
	if err == nil {
		return
	}
	r.failures = append(r.failures, err)
}

// SetWindow records the observation lookback and settled observation window.
func (r *InfraObservationResult) SetWindow(lookback time.Duration, start, end time.Time) {
	if r.windowSet {
		r.Fail(errInfraWindowAlreadySet)
		return
	}
	r.lookback = lookback
	r.windowStart = start
	r.windowEnd = end
	r.windowSet = true
}

// AddSLOCheck appends an infrastructure SLO check.
func (r *InfraObservationResult) AddSLOCheck(check *evalspb.InfraSloCheck) {
	if check == nil {
		r.Fail(errNilInfraSLOCheck)
		return
	}
	r.infraChecks = append(r.infraChecks, proto.Clone(check).(*evalspb.InfraSloCheck))
}

// AddCloudRunSnapshot appends a Cloud Run observation snapshot.
func (r *InfraObservationResult) AddCloudRunSnapshot(snapshot *evalspb.CloudRunTargetSnapshot) {
	if snapshot == nil {
		r.Fail(errNilInfraCloudRunSnapshot)
		return
	}
	r.cloudRun = append(r.cloudRun, proto.Clone(snapshot).(*evalspb.CloudRunTargetSnapshot))
}

// AddSpannerSnapshot appends a Spanner observation snapshot.
func (r *InfraObservationResult) AddSpannerSnapshot(snapshot *evalspb.SpannerTargetSnapshot) {
	if snapshot == nil {
		r.Fail(errNilInfraSpannerSnapshot)
		return
	}
	r.spanner = append(r.spanner, proto.Clone(snapshot).(*evalspb.SpannerTargetSnapshot))
}

// InfraObservationSuite defines and runs named infrastructure-observation cases.
type InfraObservationSuite struct {
	core *suiteCore
}

// NewInfraObservationSuite constructs an observation suite with a stable short name.
func NewInfraObservationSuite(name string) *InfraObservationSuite {
	return &InfraObservationSuite{core: newSuiteCore(name, branchInfraObservation)}
}

// AddCase registers a case and returns the same suite for chaining.
func (s *InfraObservationSuite) AddCase(name string, fn InfraObservationCaseFunc) *InfraObservationSuite {
	s.core.addCase(name, fn)
	return s
}

// Run executes all registered cases synchronously and materializes a Run.
func (s *InfraObservationSuite) Run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, false)
}

// RunAndPublish executes the suite and publishes the materialized Run.
func (s *InfraObservationSuite) RunAndPublish(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, true)
}
