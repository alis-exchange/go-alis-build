package evals

import (
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/loadgen"
)

// defaultProfiles holds the framework's load defaults for every mode.
//
// Worker counts follow Little's law sized for ~250ms-latency Cloud Run
// targets (in-flight ≈ QPS × latency). Warmup grows with QPS so higher
// modes exclude autoscaler ramp from the measurement window. MINIMAL is
// deliberately tiny — it is the smoke/baseline mode for CI and quick live
// checks.
var defaultProfiles = map[evalspb.RunLoadTestRequest_Mode]loadgen.Profile{
	evalspb.RunLoadTestRequest_MINIMAL: {
		QPS:         5,
		Concurrency: 2,
		Duration:    15 * time.Second,
		Warmup:      2 * time.Second,
	},
	evalspb.RunLoadTestRequest_CONSERVATIVE: {
		QPS:         25,
		Concurrency: 10,
		Duration:    30 * time.Second,
		Warmup:      5 * time.Second,
	},
	evalspb.RunLoadTestRequest_MODERATE: {
		QPS:         100,
		Concurrency: 25,
		Duration:    60 * time.Second,
		Warmup:      10 * time.Second,
	},
	evalspb.RunLoadTestRequest_HIGH: {
		QPS:         400,
		Concurrency: 100,
		Duration:    120 * time.Second,
		Warmup:      15 * time.Second,
	},
	evalspb.RunLoadTestRequest_LUDICROUS: {
		QPS:         1000,
		Concurrency: 250,
		Duration:    180 * time.Second,
		Warmup:      20 * time.Second,
	},
}

// DefaultLoadProfile returns the framework default profile for mode, or the
// zero Profile and false when mode has no default (MODE_UNSPECIFIED).
func DefaultLoadProfile(mode evalspb.RunLoadTestRequest_Mode) (loadgen.Profile, bool) {
	p, ok := defaultProfiles[mode]
	return p, ok
}

// ResolveLoadProfile returns the effective profile for mode after applying
// any suite override on top of the framework default. Overrides fully
// replace the default; there is no field-level merging (a partially
// specified override could easily produce nonsense at high modes).
func ResolveLoadProfile(mode evalspb.RunLoadTestRequest_Mode, overrides map[evalspb.RunLoadTestRequest_Mode]loadgen.Profile) (loadgen.Profile, bool) {
	if p, ok := overrides[mode]; ok {
		return p, true
	}
	return DefaultLoadProfile(mode)
}
