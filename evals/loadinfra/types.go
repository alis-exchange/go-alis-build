package loadinfra

import (
	"time"

	"go.alis.build/evals/loadgen"
)

// TargetRole classifies an infrastructure target within a case.
type TargetRole int

const (
	// RoleEntry is the primary service receiving traffic or acting as the
	// case entrypoint.
	RoleEntry TargetRole = 1 + iota
	// RoleDependency is a downstream service or datastore observed alongside
	// the entry target.
	RoleDependency
)

// CloudRunTarget declares a Cloud Run service revision scope for Monitoring queries.
type CloudRunTarget struct {
	// ID is the stable target identifier. It must be unique within a suite.
	ID string
	// Role identifies the entry target or a dependency.
	Role TargetRole
	// ProjectID is the Google Cloud project hosting the service.
	ProjectID string
	// Region is the Cloud Run region.
	Region string
	// ServiceName is the Cloud Run service name.
	ServiceName string
	// Revision filters to one revision when non-empty.
	Revision string
}

// SpannerTarget declares a Spanner instance and database scope for Monitoring
// queries. Role is always DEPENDENCY on the wire.
type SpannerTarget struct {
	// ID is the stable target identifier. It must be unique within a suite.
	ID string
	// ProjectID is the Google Cloud project hosting the instance.
	ProjectID string
	// InstanceID is the Spanner instance ID.
	InstanceID string
	// Location is the Spanner instance location.
	Location string
	// Database is the database within the instance.
	Database string
}

// Targets groups the infrastructure targets observed together.
type Targets struct {
	// CloudRun contains service targets.
	CloudRun []CloudRunTarget
	// Spanner contains database targets.
	Spanner []SpannerTarget
}

const (
	// CloudRunSettlePadding is how long after a window ends Cloud Run metrics
	// are treated as complete. Cloud Run metrics are sampled every 60s and
	// may be invisible for up to 120s
	// (https://docs.cloud.google.com/monitoring/api/metrics_gcp_p_z); the
	// padding adds one sample period on top. ObserveLookback windows end
	// this long before now when only Cloud Run targets are declared.
	CloudRunSettlePadding = 180 * time.Second
	// SpannerSettlePadding is the Spanner equivalent. Spanner
	// api/request_latencies may be invisible for up to 180s
	// (https://docs.cloud.google.com/spanner/docs/metrics); the padding adds
	// one 60s sample period on top.
	SpannerSettlePadding = 240 * time.Second
)

// ObservationWindow is the reported inclusive-start, exclusive-end interval
// attached to infrastructure snapshots.
// Monitoring reads (start, end]; queries round the window out to whole
// minutes so the edge samples are included.
type ObservationWindow struct {
	// Start is the inclusive start of the observation window.
	Start time.Time
	// End is the exclusive end of the observation window.
	End time.Time
}

func windowFromMetrics(m *loadgen.Metrics) ObservationWindow {
	if m == nil {
		return ObservationWindow{}
	}
	return ObservationWindow{Start: m.MeasurementStart, End: m.MeasurementEnd}
}

func settleDuration(targets Targets) time.Duration {
	switch {
	case len(targets.Spanner) > 0:
		return SpannerSettlePadding
	case len(targets.CloudRun) > 0:
		return CloudRunSettlePadding
	default:
		return 0
	}
}

func lookbackWindow(lookback time.Duration, now time.Time, targets Targets) ObservationWindow {
	end := now.Add(-settleDuration(targets))
	return ObservationWindow{Start: end.Add(-lookback), End: end}
}
