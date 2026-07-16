package loadinfra

import (
	"testing"
	"time"

	"go.alis.build/evals/loadgen"
)

func TestWindowFromMetrics(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	w := WindowFromMetrics(&loadgen.Metrics{MeasurementStart: start, MeasurementEnd: end})
	if w.Start != start || w.End != end {
		t.Fatalf("WindowFromMetrics=%+v, want start=%v end=%v", w, start, end)
	}
	if got := WindowFromMetrics(nil); got.Start != (time.Time{}) || got.End != (time.Time{}) {
		t.Fatalf("WindowFromMetrics(nil)=%+v, want zero window", got)
	}
}

func TestSettleDuration(t *testing.T) {
	t.Parallel()
	if got := SettleDuration(true, true); got != SpannerSettlePadding {
		t.Fatalf("both kinds: got %v want %v", got, SpannerSettlePadding)
	}
	if got := SettleDuration(true, false); got != CloudRunSettlePadding {
		t.Fatalf("cloud only: got %v want %v", got, CloudRunSettlePadding)
	}
	if got := SettleDuration(false, true); got != SpannerSettlePadding {
		t.Fatalf("spanner only: got %v want %v", got, SpannerSettlePadding)
	}
	if got := SettleDuration(false, false); got != 0 {
		t.Fatalf("none: got %v want 0", got)
	}
}

func TestWindowLookback(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	lookback := 30 * time.Minute
	settle := 90 * time.Second
	w := WindowLookback(lookback, now, settle)
	wantEnd := now.Add(-settle)
	wantStart := wantEnd.Add(-lookback)
	if w.End != wantEnd || w.Start != wantStart {
		t.Fatalf("WindowLookback=%+v, want [%v, %v)", w, wantStart, wantEnd)
	}
}

func TestQueryWindowPadding(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)
	w := ObservationWindow{Start: start, End: end}

	cr := CloudRunQueryWindow(w)
	if cr.Start != start || cr.End != end.Add(CloudRunSettlePadding) {
		t.Fatalf("CloudRunQueryWindow=%+v", cr)
	}
	sp := SpannerQueryWindow(w)
	if sp.Start != start || sp.End != end.Add(SpannerSettlePadding) {
		t.Fatalf("SpannerQueryWindow=%+v", sp)
	}
}
