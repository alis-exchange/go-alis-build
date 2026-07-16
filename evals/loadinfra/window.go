package loadinfra

import (
	"time"

	"go.alis.build/evals/loadgen"
)

const (
	// CloudRunSettlePadding is the visibility delay applied when settling
	// standalone windows or extending load-integrated query intervals for
	// Cloud Run metrics.
	CloudRunSettlePadding = 90 * time.Second
	// SpannerSettlePadding is the visibility delay applied when settling
	// standalone windows or extending load-integrated query intervals for
	// Spanner metrics.
	SpannerSettlePadding = 180 * time.Second
)

// ObservationWindow is the reported inclusive-start, exclusive-end interval
// attached to infra snapshots.
type ObservationWindow struct {
	// Start is the inclusive start of the observation window (UTC).
	Start time.Time
	// End is the exclusive end of the observation window (UTC).
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
// standalone lookback windows. When both Cloud Run and Spanner targets are
// present, the maximum (Spanner) delay applies.
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

// CloudRunQueryWindow extends the reported window end for load-integrated
// Monitoring queries so recently ingested Cloud Run data is included.
func CloudRunQueryWindow(w ObservationWindow) ObservationWindow {
	return ObservationWindow{Start: w.Start, End: w.End.Add(CloudRunSettlePadding)}
}

// SpannerQueryWindow extends the reported window end for load-integrated
// Monitoring queries so recently ingested Spanner data is included.
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
