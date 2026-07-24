package evals

import (
	"context"
	"errors"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/validation"
	"google.golang.org/protobuf/proto"
)

// LoadCaseFunc runs one load case against a result builder.
type LoadCaseFunc func(context.Context, *LoadResult)

var (
	errLoadSummaryAlreadySet   = errors.New("evals: load summary already set")
	errNilLoadSummary          = errors.New("evals: nil load summary")
	errNilLoadSLOCheck         = errors.New("evals: nil load SLO check")
	errNilLoadTag              = errors.New("evals: nil load tag")
	errNilLoadCloudRunSnapshot = errors.New("evals: nil load Cloud Run snapshot")
	errNilLoadSpannerSnapshot  = errors.New("evals: nil load Spanner snapshot")
	errNilLoadInfraSLOCheck    = errors.New("evals: nil load infra SLO check")
)

// LoadResult collects protobuf-native load-test case data.
type LoadResult struct {
	validator   *validation.Validator
	summary     *evalspb.LoadTestResults_Summary
	summarySet  bool
	checks      []*evalspb.LoadTestResults_SloCheck
	tags        []*evalspb.LoadTestResults_StringEntry
	cloudRun    []*evalspb.CloudRunTargetSnapshot
	spanner     []*evalspb.SpannerTargetSnapshot
	infraChecks []*evalspb.InfraSloCheck
	failures    []error
}

func newLoadResult() *LoadResult {
	return &LoadResult{validator: validation.NewValidator()}
}

// Validator returns the case-local validator used for general validation rules.
func (r *LoadResult) Validator() *validation.Validator {
	if r.validator == nil {
		r.validator = validation.NewValidator()
	}
	return r.validator
}

// Fail records a case failure while preserving any data already added.
func (r *LoadResult) Fail(err error) {
	if err == nil {
		return
	}
	r.failures = append(r.failures, err)
}

// SetSummary records the protobuf-native load summary for this case.
func (r *LoadResult) SetSummary(summary *evalspb.LoadTestResults_Summary) {
	if summary == nil {
		r.Fail(errNilLoadSummary)
		return
	}
	if r.summarySet {
		r.Fail(errLoadSummaryAlreadySet)
		return
	}
	r.summary = proto.Clone(summary).(*evalspb.LoadTestResults_Summary)
	r.summarySet = true
}

// AddSLOCheck appends a load SLO check.
func (r *LoadResult) AddSLOCheck(check *evalspb.LoadTestResults_SloCheck) {
	if check == nil {
		r.Fail(errNilLoadSLOCheck)
		return
	}
	r.checks = append(r.checks, proto.Clone(check).(*evalspb.LoadTestResults_SloCheck))
}

// AddTag appends an author-declared load tag.
func (r *LoadResult) AddTag(tag *evalspb.LoadTestResults_StringEntry) {
	if tag == nil {
		r.Fail(errNilLoadTag)
		return
	}
	r.tags = append(r.tags, proto.Clone(tag).(*evalspb.LoadTestResults_StringEntry))
}

// AddCloudRunSnapshot appends a Cloud Run diagnostic snapshot.
func (r *LoadResult) AddCloudRunSnapshot(snapshot *evalspb.CloudRunTargetSnapshot) {
	if snapshot == nil {
		r.Fail(errNilLoadCloudRunSnapshot)
		return
	}
	r.cloudRun = append(r.cloudRun, proto.Clone(snapshot).(*evalspb.CloudRunTargetSnapshot))
}

// AddSpannerSnapshot appends a Spanner diagnostic snapshot.
func (r *LoadResult) AddSpannerSnapshot(snapshot *evalspb.SpannerTargetSnapshot) {
	if snapshot == nil {
		r.Fail(errNilLoadSpannerSnapshot)
		return
	}
	r.spanner = append(r.spanner, proto.Clone(snapshot).(*evalspb.SpannerTargetSnapshot))
}

// AddInfraSLOCheck appends a diagnostics-only infrastructure SLO check.
func (r *LoadResult) AddInfraSLOCheck(check *evalspb.InfraSloCheck) {
	if check == nil {
		r.Fail(errNilLoadInfraSLOCheck)
		return
	}
	r.infraChecks = append(r.infraChecks, proto.Clone(check).(*evalspb.InfraSloCheck))
}

// LoadSuite defines and runs named load-test cases.
type LoadSuite struct {
	core *suiteCore
}

// NewLoadSuite constructs a load suite with a stable short name.
func NewLoadSuite(name string) *LoadSuite {
	return &LoadSuite{core: newSuiteCore(name, branchLoad)}
}

// AddCase registers a case and returns the same suite for chaining.
func (s *LoadSuite) AddCase(name string, fn LoadCaseFunc) *LoadSuite {
	s.core.addCase(name, fn)
	return s
}

// Run executes all registered cases synchronously and materializes a Run.
func (s *LoadSuite) Run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, false)
}

// RunAndPublish executes the suite and publishes the materialized Run.
func (s *LoadSuite) RunAndPublish(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, true)
}
