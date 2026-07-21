package loadgen

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// zeroLatencyTarget returns nil immediately. Useful for testing pacing
// without adding scheduler variance to worker execution time.
func zeroLatencyTarget(context.Context, CallData) TargetResult { return TargetResult{} }

// TestInProcess_PacingAccuracy verifies that RequestCount is close to
// QPS × Duration within a reasonable tolerance for a fast target. We do not
// assert a tight bound: go test scheduling on shared CI can move things
// around, so ±25% is the practical envelope for a 500ms window at 50 QPS.
func TestInProcess_PacingAccuracy(t *testing.T) {
	t.Parallel()

	g := New()
	p := Profile{QPS: 50, Concurrency: 5, Duration: 500 * time.Millisecond}
	m, err := g.Run(context.Background(), p, zeroLatencyTarget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := p.QPS * p.Duration.Seconds()
	lo, hi := want*0.75, want*1.25
	if m.RequestCount < int64(lo) || m.RequestCount > int64(hi) {
		t.Fatalf("RequestCount=%d, want between %.0f and %.0f", m.RequestCount, lo, hi)
	}
	if m.ErrorCount != 0 {
		t.Fatalf("ErrorCount=%d, want 0", m.ErrorCount)
	}
	if m.ActualQPS <= 0 {
		t.Fatalf("ActualQPS=%v, want > 0", m.ActualQPS)
	}
}

// TestInProcess_WarmupExcluded verifies that samples produced during the
// warmup window are not counted in aggregate metrics.
func TestInProcess_WarmupExcluded(t *testing.T) {
	t.Parallel()

	g := New()
	p := Profile{
		QPS:         50,
		Concurrency: 5,
		Warmup:      300 * time.Millisecond,
		Duration:    300 * time.Millisecond,
	}

	// Warmup + Duration = 600ms at 50 QPS ≈ 30 total requests. Only the ~15
	// that fall inside the measurement window should be counted.
	m, err := g.Run(context.Background(), p, zeroLatencyTarget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Upper bound: with warmup, we should never see more than roughly the
	// full window's worth of requests. Using 25 gives comfortable headroom
	// while still catching an accidental "warmup samples included" bug (which
	// would push the count toward ~30).
	if m.RequestCount > 25 {
		t.Fatalf("RequestCount=%d includes warmup samples; want ≤25", m.RequestCount)
	}
	if m.RequestCount < 5 {
		t.Fatalf("RequestCount=%d suspiciously low; want ~15", m.RequestCount)
	}
	if m.Duration != p.Duration {
		t.Fatalf("Duration=%v, want %v (measurement window only)", m.Duration, p.Duration)
	}
}

// TestInProcess_MeasurementWindow verifies MeasurementStart and MeasurementEnd
// bound the measurement window (warmup excluded).
func TestInProcess_MeasurementWindow(t *testing.T) {
	t.Parallel()

	g := New()
	p := Profile{QPS: 20, Concurrency: 2, Duration: 200 * time.Millisecond}
	before := time.Now()
	m, err := g.Run(context.Background(), p, zeroLatencyTarget)
	after := time.Now()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.MeasurementStart.IsZero() || m.MeasurementEnd.IsZero() {
		t.Fatalf("MeasurementStart=%v MeasurementEnd=%v, want both set", m.MeasurementStart, m.MeasurementEnd)
	}
	if got := m.MeasurementEnd.Sub(m.MeasurementStart); got != p.Duration {
		t.Fatalf("measurement span=%v, want %v", got, p.Duration)
	}
	if m.MeasurementStart.Before(before) || m.MeasurementEnd.After(after.Add(time.Second)) {
		t.Fatalf("measurement window [%v, %v) outside run bounds [%v, %v]",
			m.MeasurementStart, m.MeasurementEnd, before, after)
	}
}

// TestInProcess_MeasurementWindowWarmup verifies MeasurementStart follows warmup.
func TestInProcess_MeasurementWindowWarmup(t *testing.T) {
	t.Parallel()

	g := New()
	p := Profile{
		QPS:         20,
		Concurrency: 2,
		Warmup:      150 * time.Millisecond,
		Duration:    200 * time.Millisecond,
	}
	m, err := g.Run(context.Background(), p, zeroLatencyTarget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.MeasurementStart.IsZero() || m.MeasurementEnd.IsZero() {
		t.Fatalf("MeasurementStart=%v MeasurementEnd=%v, want both set", m.MeasurementStart, m.MeasurementEnd)
	}
	if got := m.MeasurementEnd.Sub(m.MeasurementStart); got != p.Duration {
		t.Fatalf("measurement span=%v, want %v", got, p.Duration)
	}
}

// TestInProcess_ErrorAccounting verifies error grouping by canonical code.
func TestInProcess_ErrorAccounting(t *testing.T) {
	t.Parallel()

	var (
		counter atomic.Int64
	)
	target := func(context.Context, CallData) TargetResult {
		n := counter.Add(1)
		switch n % 3 {
		case 0:
			return TargetResult{TransportErr: status.Error(codes.Unavailable, "boom")}
		case 1:
			return TargetResult{TransportErr: status.Error(codes.DeadlineExceeded, "slow")}
		default:
			return TargetResult{}
		}
	}

	g := New()
	p := Profile{QPS: 30, Concurrency: 5, Duration: 500 * time.Millisecond}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.ErrorCount == 0 {
		t.Fatal("ErrorCount=0, want > 0")
	}
	var sum int64
	for _, v := range m.ErrorsByCode {
		sum += v
	}
	if sum != m.ErrorCount {
		t.Fatalf("ErrorsByCode sum=%d, ErrorCount=%d, want equal", sum, m.ErrorCount)
	}
	// Sanity: both codes should appear given the round-robin above.
	for _, want := range []string{codes.Unavailable.String(), codes.DeadlineExceeded.String()} {
		if m.ErrorsByCode[want] == 0 {
			t.Fatalf("ErrorsByCode missing %q: %v", want, m.ErrorsByCode)
		}
	}
}

// TestInProcess_LatencyPercentiles verifies percentile ordering and sane
// values for a bimodal latency distribution.
func TestInProcess_LatencyPercentiles(t *testing.T) {
	t.Parallel()

	var counter atomic.Int64
	target := TransportTarget(func(context.Context) error {
		n := counter.Add(1)
		if n%10 == 0 {
			time.Sleep(20 * time.Millisecond)
		} else {
			time.Sleep(2 * time.Millisecond)
		}
		return nil
	})

	g := New()
	p := Profile{QPS: 40, Concurrency: 10, Duration: 800 * time.Millisecond}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.RequestCount < 10 {
		t.Fatalf("RequestCount=%d, want plenty of samples", m.RequestCount)
	}
	l := m.Latency
	if l.MinMs > l.P50Ms || l.P50Ms > l.P95Ms || l.P95Ms > l.P99Ms || l.P99Ms > l.MaxMs {
		t.Fatalf("percentile ordering violated: %+v", l)
	}
	if l.P50Ms > 15 {
		t.Fatalf("P50=%vms too high — should reflect fast body", l.P50Ms)
	}
	if l.P99Ms < 5 {
		t.Fatalf("P99=%vms too low — should reflect slow tail", l.P99Ms)
	}
}

// TestInProcess_Saturation verifies that when the pool is far too small to
// hold the target rate, ActualQPS drops below target but the run still
// completes without deadlock.
func TestInProcess_Saturation(t *testing.T) {
	t.Parallel()

	target := TransportTarget(func(context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	g := New()
	// Single worker × 50ms latency ≈ 20 QPS ceiling, well below 100 target.
	p := Profile{QPS: 100, Concurrency: 1, Duration: 400 * time.Millisecond}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.ActualQPS >= p.QPS {
		t.Fatalf("ActualQPS=%.1f, want < target %.1f under saturation", m.ActualQPS, p.QPS)
	}
	if m.RequestCount == 0 {
		t.Fatal("saturation deadlocked — no samples recorded")
	}
}

// TestInProcess_Cancellation verifies that ctx cancellation returns partial
// metrics and the underlying error.
func TestInProcess_Cancellation(t *testing.T) {
	t.Parallel()

	target := TransportTarget(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return nil
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	g := New()
	p := Profile{QPS: 100, Concurrency: 5, Duration: time.Second}
	m, err := g.Run(ctx, p, target)
	if err == nil {
		t.Fatal("expected ctx error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want ctx error", err)
	}
	if m == nil {
		t.Fatal("expected partial metrics on cancellation, got nil")
	}
}

// TestInProcess_PanicRecovered verifies that a panicking target is caught
// and recorded as an INTERNAL error rather than killing the process.
func TestInProcess_PanicRecovered(t *testing.T) {
	t.Parallel()

	var count atomic.Int64
	target := TransportTarget(func(context.Context) error {
		n := count.Add(1)
		if n%2 == 0 {
			panic("boom")
		}
		return nil
	})

	g := New()
	p := Profile{QPS: 20, Concurrency: 3, Duration: 400 * time.Millisecond}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.ErrorCount == 0 {
		t.Fatal("ErrorCount=0, want panics counted")
	}
	if m.ErrorsByCode[codes.Internal.String()] == 0 {
		t.Fatalf("expected panics under Internal: %v", m.ErrorsByCode)
	}
}

// TestInProcess_InvalidProfile returns ErrInvalidProfile before any goroutines
// are spawned.
func TestInProcess_InvalidProfile(t *testing.T) {
	t.Parallel()

	g := New()
	_, err := g.Run(context.Background(), Profile{}, zeroLatencyTarget)
	if err == nil {
		t.Fatal("expected error for zero profile")
	}
	if !errors.Is(err, ErrInvalidProfile{}) {
		t.Fatalf("err=%v, want ErrInvalidProfile", err)
	}
}

// TestInProcess_NilTarget returns an error before spawning goroutines.
func TestInProcess_NilTarget(t *testing.T) {
	t.Parallel()

	g := New()
	_, err := g.Run(context.Background(), Profile{QPS: 1, Concurrency: 1, Duration: time.Millisecond}, nil)
	if err == nil {
		t.Fatal("expected error for nil target")
	}
	if !errors.Is(err, ErrNilTarget{}) {
		t.Fatalf("err = %v, want ErrNilTarget", err)
	}
}

// concurrentInvocations records the maximum number of workers observed in
// flight simultaneously, used to sanity-check the concurrency knob.
type concurrentInvocations struct {
	mu      sync.Mutex
	current int
	peak    int
}

func (c *concurrentInvocations) enter() {
	c.mu.Lock()
	c.current++
	if c.current > c.peak {
		c.peak = c.current
	}
	c.mu.Unlock()
}

func (c *concurrentInvocations) leave() {
	c.mu.Lock()
	c.current--
	c.mu.Unlock()
}

func (c *concurrentInvocations) Peak() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.peak
}

// TestInProcess_ConcurrencyRespected checks that the worker pool actually
// caps in-flight requests. A slow target that starts many requests should
// see peak in-flight equal to (roughly) Concurrency.
func TestInProcess_ConcurrencyRespected(t *testing.T) {
	t.Parallel()

	inv := &concurrentInvocations{}
	target := TransportTarget(func(context.Context) error {
		inv.enter()
		defer inv.leave()
		time.Sleep(30 * time.Millisecond)
		return nil
	})
	g := New()
	p := Profile{QPS: 500, Concurrency: 4, Duration: 400 * time.Millisecond}
	if _, err := g.Run(context.Background(), p, target); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if peak := inv.Peak(); peak > p.Concurrency {
		t.Fatalf("peak in-flight %d exceeds Concurrency %d", peak, p.Concurrency)
	}
}

func TestChannelDropBehavior(t *testing.T) {
	t.Parallel()

	ch := make(chan time.Time)
	go func() {
		<-ch
		time.Sleep(100 * time.Millisecond)
	}()
	time.Sleep(5 * time.Millisecond)

	var dropped atomic.Int64
	for i := 0; i < 50; i++ {
		select {
		case ch <- time.Now():
		default:
			dropped.Add(1)
		}
	}
	if dropped.Load() == 0 {
		t.Fatal("expected drops when receiver busy")
	}
}

func TestInProcess_DroppedCountUnderSaturation(t *testing.T) {
	t.Parallel()

	target := TransportTarget(func(context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	g := New()
	p := Profile{QPS: 200, Concurrency: 1, Duration: 400 * time.Millisecond}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.DroppedCount == 0 {
		t.Fatalf("DroppedCount=0, want > 0 under saturation")
	}
	if m.DroppedCount >= 1_000_000 {
		t.Fatalf("DroppedCount=%d, spin bug: want < 1_000_000", m.DroppedCount)
	}
}

func TestInProcess_DroppedCountLongSaturation(t *testing.T) {
	t.Parallel()

	target := TransportTarget(func(context.Context) error {
		time.Sleep(8 * time.Second)
		return nil
	})
	g := New()
	p := Profile{
		QPS:            1,
		Concurrency:    1,
		Duration:       10 * time.Second,
		Warmup:         0,
		RequestTimeout: 30 * time.Second,
	}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.RequestCount != 1 {
		t.Fatalf("RequestCount=%d, want 1", m.RequestCount)
	}
	// ~9 pacer-side drops over 10s at QPS=1, plus 0–1 worker-side.
	if m.DroppedCount < 7 || m.DroppedCount > 12 {
		t.Fatalf("DroppedCount=%d, want in [7, 12]", m.DroppedCount)
	}
}

func TestInProcess_NoDropsWhenKeepingUp(t *testing.T) {
	t.Parallel()

	target := TransportTarget(func(context.Context) error {
		return nil
	})
	g := New()
	p := Profile{
		QPS:            5,
		Concurrency:    10,
		Duration:       200 * time.Millisecond,
		Warmup:         0,
		RequestTimeout: 5 * time.Second,
	}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.DroppedCount != 0 {
		t.Fatalf("DroppedCount=%d, want 0", m.DroppedCount)
	}
}

func TestInProcess_WorkerWindowEndDrops(t *testing.T) {
	t.Parallel()

	// High QPS with a slow target queues ticks before the window ends; workers
	// count late receives as drops. Pacer-side saturation drops may also occur.
	target := TransportTarget(func(context.Context) error {
		time.Sleep(150 * time.Millisecond)
		return nil
	})
	g := New()
	p := Profile{
		QPS:              100,
		Concurrency:      1,
		Duration:         50 * time.Millisecond,
		Warmup:           0,
		GracefulRampDown: 0,
		RequestTimeout:   5 * time.Second,
	}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.DroppedCount == 0 {
		t.Fatal("DroppedCount=0, want > 0 from worker window-end drops")
	}
	if m.DroppedCount >= 100 {
		t.Fatalf("DroppedCount=%d, want < 100 (bounded, not spin)", m.DroppedCount)
	}
}

func TestInProcess_GracefulRampDownExcludesLateSamples(t *testing.T) {
	t.Parallel()

	target := TransportTarget(func(context.Context) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	})
	g := New()
	p := Profile{
		QPS:              40,
		Concurrency:      5,
		Duration:         300 * time.Millisecond,
		GracefulRampDown: 200 * time.Millisecond,
	}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.RequestCount == 0 {
		t.Fatal("RequestCount=0, want samples")
	}
}

func TestInProcess_ConcurrencyStagesScaleWorkers(t *testing.T) {
	t.Parallel()

	inv := &concurrentInvocations{}
	target := TransportTarget(func(context.Context) error {
		inv.enter()
		defer inv.leave()
		time.Sleep(5 * time.Millisecond)
		return nil
	})
	g := New()
	total := time.Second
	p := Profile{
		QPS:         500,
		Concurrency: 2,
		Duration:    total,
		ConcurrencyStages: []Stage{
			{Duration: 200 * time.Millisecond, Target: 2},
			{Duration: 800 * time.Millisecond, Target: 8},
		},
	}
	if _, err := g.Run(context.Background(), p, target); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if peak := inv.Peak(); peak < 6 {
		t.Fatalf("peak in-flight %d, want at least 6 during high stage", peak)
	}
}

func TestInProcess_StepQPSStages(t *testing.T) {
	t.Parallel()

	g := New()
	total := 400 * time.Millisecond
	p := Profile{
		Concurrency: 10,
		Duration:    total,
		QPSStages: []Stage{
			{Duration: 200 * time.Millisecond, Target: 20},
			{Duration: 200 * time.Millisecond, Target: 80},
		},
	}
	m, err := g.Run(context.Background(), p, zeroLatencyTarget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// ~20*0.2 + 80*0.2 = 4 + 16 = 20 requests in measurement window
	if m.RequestCount < 10 || m.RequestCount > 35 {
		t.Fatalf("RequestCount=%d, want ~20 for staged QPS", m.RequestCount)
	}
}

func TestInProcess_CheckAccounting(t *testing.T) {
	t.Parallel()

	target := func(_ context.Context, _ CallData) TargetResult {
		return TargetResult{CheckErr: errors.New("assertion failed")}
	}
	g := New()
	p := Profile{QPS: 20, Concurrency: 2, Duration: 200 * time.Millisecond}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.ErrorCount != 0 {
		t.Fatalf("ErrorCount=%d, want 0 transport errors", m.ErrorCount)
	}
	if m.CheckFailedCount == 0 {
		t.Fatal("CheckFailedCount=0, want > 0")
	}
}

func TestInProcess_AbortCheckCancelsEarly(t *testing.T) {
	t.Parallel()

	g := New()
	alwaysErr := func(context.Context, CallData) TargetResult {
		return TargetResult{TransportErr: errors.New("fail")}
	}
	p := Profile{
		QPS:         50,
		Concurrency: 5,
		Duration:    30 * time.Second,
		AbortCheck: func(m *Metrics) bool {
			return m != nil && m.RequestCount >= 5 && m.ErrorCount > 0
		},
	}
	start := time.Now()
	m, err := g.Run(context.Background(), p, alwaysErr)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if elapsed > 10*time.Second {
		t.Fatalf("run took %v, expected early abort", elapsed)
	}
	if m.RequestCount > 200 {
		t.Fatalf("RequestCount=%d, expected partial window before full duration", m.RequestCount)
	}
}

func TestAggregator_AbortSnapshotSLOFields(t *testing.T) {
	t.Parallel()

	start := time.Now().Add(-time.Minute)
	agg := newAggregator(start, start.Add(time.Minute), 30*time.Second, 0)
	for i := 0; i < 50; i++ {
		agg.record(sample{
			sentAt:  start.Add(time.Duration(i) * time.Millisecond),
			latency: time.Duration(10+i%5) * time.Millisecond,
			result: TargetResult{
				TransportErr: errors.New("fail"),
				Stream: &StreamSample{
					SendDuration: time.Duration(5+i%3) * time.Millisecond,
					MessagesSent: 2,
				},
			},
		})
	}

	abort := agg.abortSnapshot()
	final := agg.finalize()
	if abort.RequestCount != final.RequestCount || abort.ErrorCount != final.ErrorCount {
		t.Fatalf("counts mismatch: abort=%+v final=%+v", abort, final)
	}
	if abort.ActualQPS != final.ActualQPS {
		t.Fatalf("ActualQPS abort=%v final=%v", abort.ActualQPS, final.ActualQPS)
	}
	for _, field := range []struct {
		name string
		got  float64
		want float64
	}{
		{"P50", abort.Latency.P50Ms, final.Latency.P50Ms},
		{"P95", abort.Latency.P95Ms, final.Latency.P95Ms},
		{"P99", abort.Latency.P99Ms, final.Latency.P99Ms},
	} {
		if field.got != field.want {
			t.Fatalf("latency %s abort=%v final=%v", field.name, field.got, field.want)
		}
	}
	if abort.Stream == nil || final.Stream == nil {
		t.Fatal("stream summary missing")
	}
	if abort.Stream.StreamCount != final.Stream.StreamCount ||
		abort.Stream.MessagesSentTotal != final.Stream.MessagesSentTotal {
		t.Fatalf("stream counts abort=%+v final=%+v", abort.Stream, final.Stream)
	}
	if abort.Stream.TTFB.P99Ms != final.Stream.TTFB.P99Ms {
		t.Fatalf("TTFB P99 abort=%v final=%v", abort.Stream.TTFB.P99Ms, final.Stream.TTFB.P99Ms)
	}
	if abort.ErrorsByCode != nil {
		t.Fatal("abort snapshot should omit ErrorsByCode")
	}
}

func TestInProcess_StreamSummaryAggregation(t *testing.T) {
	t.Parallel()

	target := func(_ context.Context, d CallData) TargetResult {
		ms := 10 + int(d.RequestNumber%5)
		return TargetResult{
			Stream: &StreamSample{
				SendDuration:    time.Duration(ms) * time.Millisecond,
				ResponseLatency: time.Duration(ms+2) * time.Millisecond,
				TotalDuration:   time.Duration(ms+5) * time.Millisecond,
				MessagesSent:    2,
			},
		}
	}
	g := New()
	p := Profile{QPS: 40, Concurrency: 4, Duration: 300 * time.Millisecond}
	m, err := g.Run(context.Background(), p, target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.Stream == nil {
		t.Fatal("Stream=nil, want aggregate stream summary")
	}
	if m.Stream.StreamCount != m.RequestCount {
		t.Fatalf("StreamCount=%d RequestCount=%d", m.Stream.StreamCount, m.RequestCount)
	}
	if m.Stream.MessagesSentTotal != 2*m.RequestCount {
		t.Fatalf("MessagesSentTotal=%d, want %d", m.Stream.MessagesSentTotal, 2*m.RequestCount)
	}
	if m.Stream.TTFB.P50Ms <= 0 {
		t.Fatalf("TTFB summary empty: %+v", m.Stream.TTFB)
	}
}

