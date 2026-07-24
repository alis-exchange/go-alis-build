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

const (
	// CloudRunSettlePadding is the visibility delay applied when settling
	// standalone windows or extending load-integrated query intervals.
	CloudRunSettlePadding = 90 * time.Second
	// SpannerSettlePadding is the equivalent Spanner visibility delay.
	SpannerSettlePadding = 180 * time.Second
)

// ObservationWindow is the reported inclusive-start, exclusive-end interval
// attached to infrastructure snapshots.
type ObservationWindow struct {
	// Start is the inclusive start of the observation window.
	Start time.Time
	// End is the exclusive end of the observation window.
	End time.Time
}

// WindowFromMetrics derives the load-integrated observation window from
// loadgen measurement timestamps (warmup excluded).
func WindowFromMetrics(m *loadgen.Metrics) ObservationWindow {
	if m == nil {
		return ObservationWindow{}
	}
	return ObservationWindow{Start: m.MeasurementStart, End: m.MeasurementEnd}
}

// SettleDuration returns the per-kind visibility delay used when settling
// standalone lookback windows. When both target kinds are present, the larger
// Spanner delay applies.
func SettleDuration(hasCloudRun, hasSpanner bool) time.Duration {
	switch {
	case hasSpanner:
		return SpannerSettlePadding
	case hasCloudRun:
		return CloudRunSettlePadding
	default:
		return 0
	}
}

// WindowLookback resolves a settled standalone observation window:
// window_end = now - settle, window_start = window_end - lookback.
func WindowLookback(lookback time.Duration, now time.Time, settle time.Duration) ObservationWindow {
	end := now.Add(-settle)
	return ObservationWindow{Start: end.Add(-lookback), End: end}
}

// CloudRunQueryWindow extends the reported window end so recently ingested
// Cloud Run data is included.
func CloudRunQueryWindow(w ObservationWindow) ObservationWindow {
	return ObservationWindow{Start: w.Start, End: w.End.Add(CloudRunSettlePadding)}
}

// SpannerQueryWindow extends the reported window end so recently ingested
// Spanner data is included.
func SpannerQueryWindow(w ObservationWindow) ObservationWindow {
	return ObservationWindow{Start: w.Start, End: w.End.Add(SpannerSettlePadding)}
}

func cloudRunQueryWindow(w ObservationWindow, extend bool) ObservationWindow {
	if extend {
		return CloudRunQueryWindow(w)
	}
	return w
}

func spannerQueryWindow(w ObservationWindow, extend bool) ObservationWindow {
	if extend {
		return SpannerQueryWindow(w)
	}
	return w
}
