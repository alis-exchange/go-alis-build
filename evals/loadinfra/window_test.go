package loadinfra

import (
	"testing"
	"time"

	"go.alis.build/evals/loadgen"
)

func TestWindowFromMetrics_internalMapping(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	w := windowFromMetrics(&loadgen.Metrics{MeasurementStart: start, MeasurementEnd: end})
	if w.Start != start || w.End != end {
		t.Fatalf("WindowFromMetrics=%+v, want start=%v end=%v", w, start, end)
	}
	if got := windowFromMetrics(nil); got.Start != (time.Time{}) || got.End != (time.Time{}) {
		t.Fatalf("windowFromMetrics(nil)=%+v, want zero window", got)
	}
}

func TestSettleDuration_internalTargetRules(t *testing.T) {
	t.Parallel()
	if got := settleDuration(Targets{CloudRun: []CloudRunTarget{{}}, Spanner: []SpannerTarget{{}}}); got != SpannerSettlePadding {
		t.Fatalf("both kinds: got %v want %v", got, SpannerSettlePadding)
	}
	if got := settleDuration(Targets{CloudRun: []CloudRunTarget{{}}}); got != CloudRunSettlePadding {
		t.Fatalf("cloud only: got %v want %v", got, CloudRunSettlePadding)
	}
	if got := settleDuration(Targets{Spanner: []SpannerTarget{{}}}); got != SpannerSettlePadding {
		t.Fatalf("spanner only: got %v want %v", got, SpannerSettlePadding)
	}
	if got := settleDuration(Targets{}); got != 0 {
		t.Fatalf("none: got %v want 0", got)
	}
}

func TestWindowLookback_internalMapping(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	lookback := 30 * time.Minute
	targets := Targets{CloudRun: []CloudRunTarget{{}}}
	w := lookbackWindow(lookback, now, targets)
	wantEnd := now.Add(-CloudRunSettlePadding)
	wantStart := wantEnd.Add(-lookback)
	if w.End != wantEnd || w.Start != wantStart {
		t.Fatalf("WindowLookback=%+v, want [%v, %v)", w, wantStart, wantEnd)
	}
}

func TestQueryWindow_roundsOutToWholeMinutes(t *testing.T) {
	t.Parallel()
	at := func(h, m, s int) time.Time { return time.Date(2026, 7, 16, h, m, s, 0, time.UTC) }
	tests := []struct {
		name string
		in   ObservationWindow
		want ObservationWindow
	}{
		{"inside minutes", ObservationWindow{at(10, 0, 20), at(10, 5, 40)}, ObservationWindow{at(10, 0, 0), at(10, 6, 0)}},
		{"on boundaries", ObservationWindow{at(10, 0, 0), at(10, 5, 0)}, ObservationWindow{at(10, 0, 0), at(10, 5, 0)}},
		{"empty on boundary", ObservationWindow{at(10, 0, 0), at(10, 0, 0)}, ObservationWindow{at(10, 0, 0), at(10, 1, 0)}},
		{"empty inside minute", ObservationWindow{at(10, 0, 30), at(10, 0, 30)}, ObservationWindow{at(10, 0, 0), at(10, 1, 0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := queryWindow(tt.in); !got.Start.Equal(tt.want.Start) || !got.End.Equal(tt.want.End) {
				t.Fatalf("queryWindow(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSettlePaddings_coverDocumentedVisibilityDelays(t *testing.T) {
	t.Parallel()
	if CloudRunSettlePadding != 180*time.Second {
		t.Fatalf("CloudRunSettlePadding = %v, want 3m0s", CloudRunSettlePadding)
	}
	if SpannerSettlePadding != 240*time.Second {
		t.Fatalf("SpannerSettlePadding = %v, want 4m0s", SpannerSettlePadding)
	}
}
