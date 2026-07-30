package loadgen

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
	"go.alis.build/alog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Generator runs one load window against a target function. Implementations
// must be safe to call sequentially against different profiles.
type Generator interface {
	Run(ctx context.Context, p Profile, target ResultTarget) (*Metrics, error)
}

// Run executes one load window using the default in-process generator.
func Run(ctx context.Context, p Profile, target ResultTarget) (*Metrics, error) {
	return New().Run(ctx, p, target)
}

// New returns the default in-process Generator. Each Run spawns its own
// goroutines; there is no shared state between runs.
func New() Generator { return &inProcess{} }

// saturationThreshold is the actual/target ratio below which we log a
// warning after the window closes. Chosen so brief pacing hiccups don't
// nag but sustained undershoot does.
const saturationThreshold = 0.9

// inProcess is the default [Generator] implementation; each Run is isolated.
type inProcess struct{}

// hdrConfig covers 1µs to 6h with 3 significant figures. Load iterations can
// legitimately run for hours (multi-GB upload pipelines), so the ceiling must
// sit above any authored RequestTimeout; values beyond it are clamped to the
// max, which silently flattens tail percentiles. HDR memory grows with the
// log of the range, so the wider bound costs little.
const (
	hdrMinValueUs = 1                                       // minimum recordable latency in microseconds
	hdrMaxValueUs = int64(6 * time.Hour / time.Microsecond) // maximum recordable latency in microseconds
	hdrSigFigs    = 3                                       // HDR histogram significant-figure precision
)

// sample is one completed request handed from a worker to the aggregator.
type sample struct {
	// sentAt is the pacer tick timestamp used for warmup/window filtering.
	sentAt time.Time
	// latency is wall time from tick dispatch through target return.
	latency time.Duration
	// result holds transport, check, and optional stream outcome.
	result TargetResult
}

// Run executes the load window described by p, driving target via a worker
// pool paced by the profile's selected pacer — or, when p.ClosedLoop is set,
// via self-dispatching workers that call the target back to back.
//
// Transport failures increment Metrics.ErrorCount and ErrorsByCode; semantic
// check failures increment CheckFailedCount. Neither aborts the window. The
// one exception is failures the generator itself caused by cutting a call
// short at the window boundary — those count as DroppedCount, not errors (see
// runCall). Only ctx cancellation, an abort check, and invalid profile
// produce a returned error (with partial metrics for cancellation/abort).
func (g *inProcess) Run(ctx context.Context, p Profile, target ResultTarget) (*Metrics, error) {
	if target == nil {
		return nil, ErrNilTarget{}
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}

	total := p.Warmup + p.Duration
	maxWorkers := p.MaxConcurrency()
	rampDown := p.resolvedGracefulRampDown(total)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The tick channel and in-flight gauge exist only for open-loop (paced)
	// dispatch; closed-loop workers self-dispatch, so in that mode ticks stays
	// nil and inFlight idle rather than being built and never read. dropped is
	// shared: both modes exclude boundary-truncated failures via runCall.
	var ticks chan time.Time
	if !p.ClosedLoop {
		ticks = make(chan time.Time, maxWorkers)
	}
	samples := make(chan sample, maxWorkers*4)
	var dropped atomic.Int64
	var inFlight atomic.Int32

	start := time.Now()
	windowEnd := start.Add(total)
	measurementStart := start.Add(p.Warmup)
	measurementEnd := measurementStart.Add(p.Duration)
	agg := newAggregator(measurementStart, measurementEnd, p.Duration, 0)
	aggDone := make(chan *Metrics, 1)
	go func() {
		aggDone <- agg.consume(samples)
	}()

	if p.AbortCheck != nil {
		go runAbortWatcher(runCtx, cancel, p.AbortCheck, agg)
	}

	reqTimeout := p.resolvedRequestTimeout()
	var wg sync.WaitGroup
	var workerMu sync.Mutex
	stops := make([]context.CancelFunc, 0, maxWorkers)
	var reqNum atomic.Uint64

	startWorker := func(workerID int) {
		workerCtx, cancel := context.WithCancel(runCtx)
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if p.ClosedLoop {
				runClosedWorker(workerCtx, samples, &dropped, target, reqTimeout, windowEnd, rampDown, &reqNum, id)
				return
			}
			runWorker(workerCtx, ticks, samples, target, reqTimeout, windowEnd, rampDown, &inFlight, &reqNum, &dropped, id)
		}(workerID)
		workerMu.Lock()
		stops = append(stops, cancel)
		workerMu.Unlock()
	}

	stopWorker := func() {
		workerMu.Lock()
		defer workerMu.Unlock()
		if len(stops) == 0 {
			return
		}
		last := len(stops) - 1
		stops[last]()
		stops = stops[:last]
	}

	for i := 0; i < p.initialConcurrency(); i++ {
		startWorker(i)
	}

	maxConc := int32(p.MaxConcurrency())

	if len(p.ConcurrencyStages) > 0 {
		var nextWorkerID atomic.Int32
		nextWorkerID.Store(int32(p.initialConcurrency()))
		addWorker := func() {
			id := int(nextWorkerID.Add(1))
			startWorker(id)
		}
		go runConcurrencySupervisor(runCtx, start, p.ConcurrencyStages, addWorker, stopWorker, func() int {
			workerMu.Lock()
			defer workerMu.Unlock()
			return len(stops)
		})
	}

	pacerDone := make(chan struct{})
	if p.ClosedLoop {
		// Worker-driven dispatch: closed-loop workers loop until the window
		// closes, so there is no pacer — this goroutine only marks the window
		// boundary for the shared ramp-down sequence below. The timer is
		// anchored to windowEnd rather than goroutine startup so that worker
		// spawning and scheduler delay between start and this goroutine
		// running cannot push the boundary (and with it the whole
		// Warmup+Duration+GracefulRampDown wall-time bound) past its schedule.
		go func() {
			defer close(pacerDone)
			timer := time.NewTimer(time.Until(windowEnd))
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-runCtx.Done():
			}
		}()
	} else {
		pacer := pacerForProfile(p, total)
		go func() {
			defer close(pacerDone)
			defer close(ticks)
			runPacerLoop(runCtx, pacer, start, total, ticks, maxConc, &inFlight, &dropped)
		}()
	}

	<-pacerDone

	// Ramp-down is always bounded: in-flight calls get up to rampDown past the
	// window boundary (their per-call contexts expire on the same schedule —
	// see runWorker), then remaining workers are cancelled. A bare unbounded
	// wg.Wait() here previously let one late iteration stall the run for a
	// full extra request-timeout.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	if runCtx.Err() != nil {
		// The run was cancelled or aborted: in-flight call contexts are
		// already dead, so skip the ramp-down grace and cancel workers now.
		workerMu.Lock()
		for _, cancel := range stops {
			cancel()
		}
		workerMu.Unlock()
		<-done
	} else {
		rampTimer := time.NewTimer(rampDown)
		select {
		case <-done:
			rampTimer.Stop()
		case <-rampTimer.C:
			workerMu.Lock()
			for _, cancel := range stops {
				cancel()
			}
			workerMu.Unlock()
			<-done
		}
	}

	close(samples)
	m := <-aggDone
	m.DroppedCount = dropped.Load()
	m.WallDuration = time.Since(start)

	if ctxErr := runCtx.Err(); ctxErr != nil {
		return m, ctxErr
	}

	// The schedule ran to completion: report the configured measurement window
	// (the pacer no longer sleeps past the boundary, so the aggregator may
	// finalize up to one scheduling interval early). Cancelled and aborted
	// runs return above with elapsed-based partial numbers instead.
	m.Duration = p.Duration
	m.ActualQPS = float64(m.RequestCount) / p.Duration.Seconds()

	// Post-run diagnostics are mode-specific. A closed-loop run has no target
	// rate — the achieved rate is the target by construction — so the
	// actual-vs-target saturation comparison is meaningless there and each
	// mode gets its own zero-sample explanation.
	if p.ClosedLoop {
		if m.RequestCount == 0 {
			// Closed-loop workers dispatch immediately, so an empty window
			// means every iteration outlived Duration and was cut off at the
			// boundary (those cut-offs count as dropped, not errors).
			alog.Warnf(runCtx, "loadgen recorded zero in-window samples: closed loop with concurrency=%d over %s window (dropped=%d) — every iteration outlived the window; lengthen Duration or GracefulRampDown",
				p.MaxConcurrency(), p.Duration, m.DroppedCount)
		}
	} else {
		targetQPS := p.EffectiveQPS()
		if m.RequestCount == 0 {
			// The worst degenerate window: nothing was recorded at all. Slot 0
			// always dispatches at t=0, so this means the dispatched work never
			// became an in-window sample: the call outlived the window (counted
			// as dropped), Warmup swallowed every dispatched slot, or
			// saturation dropped every tick. ActualQPS is 0 here, so the
			// saturation warning below would stay silent without this branch.
			alog.Warnf(runCtx, "loadgen recorded zero in-window samples: target %.2f qps over %s window (dropped=%d) — Duration is likely shorter than one iteration, or Warmup consumed every dispatched slot",
				targetQPS, p.Duration, m.DroppedCount)
		} else if m.ActualQPS < saturationThreshold*targetQPS {
			alog.Warnf(runCtx, "loadgen saturated: actual %.1f qps < target %.1f qps (concurrency=%d, mean latency %.1fms)",
				m.ActualQPS, targetQPS, p.MaxConcurrency(), m.Latency.MeanMs)
		}
	}
	return m, nil
}

// abortPollInterval controls how often [Profile.AbortCheck] is evaluated.
// Tests may shorten it via setAbortPollIntervalForTest.
var abortPollInterval = 2 * time.Second

func setAbortPollIntervalForTest(d time.Duration) func() {
	prev := abortPollInterval
	abortPollInterval = d
	return func() { abortPollInterval = prev }
}

// runAbortWatcher polls AbortCheck every abortPollInterval and cancels the run when it returns true.
func runAbortWatcher(ctx context.Context, cancel context.CancelFunc, check AbortCheck, agg *aggregator) {
	ticker := time.NewTicker(abortPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if check(agg.abortSnapshot()) {
				cancel()
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// aggregator folds worker samples into HDR histograms and counters for one window.
type aggregator struct {
	// measurementStart is the inclusive measurement boundary; warmup samples are dropped.
	measurementStart time.Time
	// measurementEnd is the exclusive measurement boundary.
	measurementEnd time.Time
	// measureFor is the Duration field copied into finalized Metrics.
	measureFor time.Duration
	// dropped counts pacer-side drops seeded at construction; worker drops added later.
	dropped int64

	// mu guards all counters and histograms below.
	mu sync.Mutex
	// hist records per-request end-to-end latency.
	hist *hdrhistogram.Histogram
	// ttfbHist records stream send-phase latency.
	ttfbHist *hdrhistogram.Histogram
	// respHist records stream response latency.
	respHist *hdrhistogram.Histogram
	// totalHist records stream total duration.
	totalHist *hdrhistogram.Histogram
	// errorsByCode groups transport failures by gRPC code name.
	errorsByCode map[string]int64
	// count is requests recorded inside the measurement window.
	count int64
	// errCount is transport failures among recorded requests.
	errCount int64
	// checkPassed counts semantic checks that passed.
	checkPassed int64
	// checkFailed counts semantic check failures.
	checkFailed int64
	// latencySumUs is the sum of latencies in microseconds for mean calculation.
	latencySumUs float64
	// streamCount is requests that returned StreamSample data.
	streamCount int64
	// messagesTotal is total stream messages sent across all streams.
	messagesTotal int64
}

// newAggregator constructs an empty aggregator for one measurement window.
func newAggregator(measurementStart, measurementEnd time.Time, measureFor time.Duration, dropped int64) *aggregator {
	return &aggregator{
		measurementStart: measurementStart,
		measurementEnd:   measurementEnd,
		measureFor:       measureFor,
		dropped:          dropped,
		hist:             hdrhistogram.New(hdrMinValueUs, hdrMaxValueUs, hdrSigFigs),
		ttfbHist:         hdrhistogram.New(hdrMinValueUs, hdrMaxValueUs, hdrSigFigs),
		respHist:         hdrhistogram.New(hdrMinValueUs, hdrMaxValueUs, hdrSigFigs),
		totalHist:        hdrhistogram.New(hdrMinValueUs, hdrMaxValueUs, hdrSigFigs),
		errorsByCode:     map[string]int64{},
	}
}

// consume drains samples until the channel closes and returns finalized metrics.
func (a *aggregator) consume(samples <-chan sample) *Metrics {
	for s := range samples {
		a.record(s)
	}
	return a.finalize()
}

// abortSnapshot returns partial metrics for mid-run SLO checks. It omits
// fields abort checks never read (ErrorsByCode, check counts, dropped) and
// skips min/max/mean latency plus non-TTFB stream histograms.
func (a *aggregator) abortSnapshot() *Metrics {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.buildAbortMetrics()
}

// finalize builds complete Metrics from accumulated samples under a lock.
func (a *aggregator) finalize() *Metrics {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.buildMetrics()
}

// record ingests one sample when its sentAt falls inside the measurement window.
func (a *aggregator) record(s sample) {
	if a.measurementStart.IsZero() || s.sentAt.Before(a.measurementStart) {
		return
	}
	if !a.measurementEnd.IsZero() && !s.sentAt.Before(a.measurementEnd) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	a.count++
	us := s.latency.Microseconds()
	if us < hdrMinValueUs {
		us = hdrMinValueUs
	}
	if us > hdrMaxValueUs {
		us = hdrMaxValueUs
	}
	_ = a.hist.RecordValue(us)
	a.latencySumUs += float64(s.latency) / float64(time.Microsecond)
	if s.result.TransportErr != nil {
		a.errCount++
		a.errorsByCode[errorCode(s.result.TransportErr)]++
	}
	if s.result.CheckErr != nil {
		a.checkFailed++
	} else if s.result.TransportErr == nil {
		a.checkPassed++
	}
	if st := s.result.Stream; st != nil {
		a.streamCount++
		a.messagesTotal += int64(st.MessagesSent)
		recordDurationHist(a.ttfbHist, st.SendDuration)
		recordDurationHist(a.respHist, st.ResponseLatency)
		recordDurationHist(a.totalHist, st.TotalDuration)
	}
}

// buildMetrics assembles the full Metrics snapshot from aggregator state.
func (a *aggregator) buildMetrics() *Metrics {
	elapsed := a.measurementElapsed()
	m := &Metrics{
		Duration:         elapsed,
		RequestCount:     a.count,
		ErrorCount:       a.errCount,
		CheckPassedCount: a.checkPassed,
		CheckFailedCount: a.checkFailed,
		ErrorsByCode:     cloneErrorsMap(a.errorsByCode),
		DroppedCount:     a.dropped,
		MeasurementStart: a.measurementStart,
		MeasurementEnd:   a.measurementEnd,
	}
	if elapsed > 0 {
		m.ActualQPS = float64(a.count) / elapsed.Seconds()
	}
	if a.count > 0 {
		m.Latency = latencyFromHist(a.hist, a.latencySumUs, a.count)
	}
	if a.streamCount > 0 {
		m.Stream = &StreamSummary{
			StreamCount:       a.streamCount,
			MessagesSentTotal: a.messagesTotal,
			TTFB:              latencyFromHist(a.ttfbHist, 0, a.streamCount),
			ResponseLatency:   latencyFromHist(a.respHist, 0, a.streamCount),
			TotalDuration:     latencyFromHist(a.totalHist, 0, a.streamCount),
		}
	}
	return m
}

// buildAbortMetrics assembles the reduced Metrics snapshot used by AbortCheck.
func (a *aggregator) buildAbortMetrics() *Metrics {
	elapsed := a.measurementElapsed()
	m := &Metrics{
		Duration:     elapsed,
		RequestCount: a.count,
		ErrorCount:   a.errCount,
	}
	if elapsed > 0 {
		m.ActualQPS = float64(a.count) / elapsed.Seconds()
	}
	if a.count > 0 {
		m.Latency = latencyPercentilesFromHist(a.hist)
	}
	if a.streamCount > 0 {
		m.Stream = &StreamSummary{
			StreamCount:       a.streamCount,
			MessagesSentTotal: a.messagesTotal,
			TTFB:              latencyPercentilesFromHist(a.ttfbHist),
		}
	}
	return m
}

func (a *aggregator) measurementElapsed() time.Duration {
	end := time.Now()
	if !a.measurementEnd.IsZero() && end.After(a.measurementEnd) {
		end = a.measurementEnd
	}
	d := end.Sub(a.measurementStart)
	if d <= 0 {
		return time.Nanosecond
	}
	return d
}

// recordDurationHist records d into h, clamping to the shared HDR value range.
func recordDurationHist(h *hdrhistogram.Histogram, d time.Duration) {
	if d <= 0 {
		return
	}
	us := d.Microseconds()
	if us < hdrMinValueUs {
		us = hdrMinValueUs
	}
	if us > hdrMaxValueUs {
		us = hdrMaxValueUs
	}
	_ = h.RecordValue(us)
}

// latencyPercentilesFromHist derives P50/P95/P99 from an HDR histogram in milliseconds.
func latencyPercentilesFromHist(h *hdrhistogram.Histogram) LatencySummary {
	if h.TotalCount() == 0 {
		return LatencySummary{}
	}
	return LatencySummary{
		P50Ms: usToMs(h.ValueAtQuantile(50)),
		P95Ms: usToMs(h.ValueAtQuantile(95)),
		P99Ms: usToMs(h.ValueAtQuantile(99)),
	}
}

// latencyFromHist builds a full LatencySummary including min, max, and mean.
func latencyFromHist(h *hdrhistogram.Histogram, sumUs float64, count int64) LatencySummary {
	if count == 0 {
		return LatencySummary{}
	}
	summary := latencyPercentilesFromHist(h)
	summary.MinMs = usToMs(h.Min())
	summary.MaxMs = usToMs(h.Max())
	summary.MeanMs = sumUs / float64(count) / 1000.0
	if sumUs == 0 {
		summary.MeanMs = usToMs(int64(h.Mean()))
	}
	return summary
}

// cloneErrorsMap returns a defensive copy of errorsByCode for Metrics output.
func cloneErrorsMap(in map[string]int64) map[string]int64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// runConcurrencySupervisor adjusts the worker pool every 50ms to match staged
// ConcurrencyStages targets for the elapsed window time.
func runConcurrencySupervisor(
	ctx context.Context,
	start time.Time,
	stages []Stage,
	add, remove func(),
	current func() int,
) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			want := concurrencyAt(time.Since(start), stages)
			for current() < want {
				add()
			}
			for current() > want {
				remove()
			}
		}
	}
}

// concurrencyAt returns the configured worker count for elapsed time within stages.
func concurrencyAt(elapsed time.Duration, stages []Stage) int {
	offset := time.Duration(0)
	for _, s := range stages {
		if elapsed < offset+s.Duration {
			return int(s.Target)
		}
		offset += s.Duration
	}
	if len(stages) == 0 {
		return 1
	}
	return int(stages[len(stages)-1].Target)
}

// runPacerLoop dispatches ticks on the pacer's schedule until the window
// closes. Extracted from Run so the closed-loop branch can skip it entirely.
func runPacerLoop(runCtx context.Context, pacer Pacer, start time.Time, total time.Duration, ticks chan<- time.Time, maxConc int32, inFlight *atomic.Int32, dropped *atomic.Int64) {
	var sent uint64
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		elapsed := time.Since(start)
		wait, stop := pacer.Pace(elapsed, sent)
		// Clamp scheduling to the window: a slot that lands at or past the
		// boundary must not be dispatched. Without this the pacer sleeps up
		// to 1/QPS past the end and dispatches one extra out-of-window tick
		// whose execution is excluded from aggregates but still blocks
		// ramp-down — pure wasted wall time.
		if stop || elapsed+wait >= total {
			return
		}
		if wait > 0 {
			timer.Reset(wait)
			select {
			case <-timer.C:
			case <-runCtx.Done():
				return
			}
		}
		now := time.Now()
		if now.Sub(start) >= total {
			return
		}
		if inFlight.Load() >= maxConc {
			// Drop when concurrency is saturated (open-loop). Advance sent so
			// Pace schedules the next slot instead of spinning on wait==0.
			dropped.Add(1)
			sent++
		} else {
			select {
			case ticks <- now:
				sent++
				inFlight.Add(1)
			default:
				// Drop when the tick channel is full; advance sent like saturation.
				dropped.Add(1)
				sent++
			}
		}
		select {
		case <-runCtx.Done():
			return
		default:
		}
	}
}

// runClosedWorker executes the target back to back until the window closes,
// keeping this worker permanently in flight — the closed-loop saturation
// model. Per-call budget and boundary-truncation semantics are shared with
// runWorker via runCall, so the run-level ramp-down bound holds unchanged.
// Dispatch never drops: a new call starts exactly when the previous one
// finishes; only boundary-truncated failures are excluded (as dropped).
func runClosedWorker(parent context.Context, samples chan<- sample, dropped *atomic.Int64, target ResultTarget, reqTimeout time.Duration, windowEnd time.Time, rampDown time.Duration, reqNum *atomic.Uint64, workerID int) {
	for {
		if parent.Err() != nil {
			return
		}
		now := time.Now()
		remaining := windowEnd.Sub(now)
		if remaining <= 0 {
			return
		}
		runCall(parent, samples, dropped, target, reqTimeout, remaining, rampDown, windowEnd, reqNum, workerID, now)
	}
}

// runWorker pulls ticks and executes the target with a per-request timeout.
// Panics in the target are recovered and recorded as INTERNAL errors so the
// window keeps running.
func runWorker(parent context.Context, ticks <-chan time.Time, samples chan<- sample, target ResultTarget, reqTimeout time.Duration, windowEnd time.Time, rampDown time.Duration, inFlight *atomic.Int32, reqNum *atomic.Uint64, dropped *atomic.Int64, workerID int) {
	for sentAt := range ticks {
		if err := parent.Err(); err != nil {
			dropped.Add(1)
			inFlight.Add(-1)
			return
		}
		// remaining is measured against the absolute window end, not the
		// tick's own age: a tick dequeued late in the window must NOT get a
		// fresh full budget, or tail iterations run a whole extra window past
		// the boundary.
		remaining := time.Until(windowEnd)
		if remaining <= 0 {
			// Ticks dispatched before the boundary but picked up after it are
			// excluded from aggregates; count as dropped so ActualQPS stays
			// honest under ramp-down.
			dropped.Add(1)
			inFlight.Add(-1)
			continue
		}
		runCall(parent, samples, dropped, target, reqTimeout, remaining, rampDown, windowEnd, reqNum, workerID, sentAt)
		inFlight.Add(-1)
	}
}

// runCall executes one target invocation under the per-call budget shared by
// both dispatch modes: a call never gets more than min(RequestTimeout,
// remaining window + ramp-down grace), so a call dispatched near the boundary
// may finish inside the bounded ramp-down grace but can never outrun the
// run-level ramp-down timer.
//
// A failure caused by that boundary cap is counted as dropped rather than
// recorded: it says nothing about the target, the run simply ended mid-call.
// Without this exclusion a healthy-but-slow service accrues one structural
// DEADLINE_EXCEEDED per mid-flight worker at every window close (all of them,
// in closed-loop mode), which is enough to trip an error-rate SLO gate.
//
// The exclusion is deliberately narrow — all four must hold:
//
//   - the budget was truncated below the profile's own RequestTimeout, so a
//     full-budget expiry (a genuine target timeout) can never match;
//   - the call's own context had ended (reqCtx.Err() captured before cancel),
//     so a fast target that merely returns a DEADLINE_EXCEEDED status of its
//     own is still recorded as a real error;
//   - the failure has deadline/cancellation semantics, so any other status
//     (UNAVAILABLE, INTERNAL, ...) surfacing at the boundary is still
//     recorded, as is a success that squeaked in under the truncated budget;
//   - the window had already closed when the call ended, so aborts and user
//     cancellations mid-window keep their partial error picture intact. This
//     also covers the ramp-down cutoff racing the truncated deadline: both
//     end the same call at windowEnd+rampDown, both are generator-induced,
//     and both must land in DroppedCount.
func runCall(parent context.Context, samples chan<- sample, dropped *atomic.Int64, target ResultTarget, reqTimeout, remaining, rampDown time.Duration, windowEnd time.Time, reqNum *atomic.Uint64, workerID int, sentAt time.Time) {
	timeout := reqTimeout
	truncated := false
	if budget := remaining + rampDown; budget < timeout {
		timeout = budget
		truncated = true
	}
	reqCtx, cancel := context.WithTimeout(parent, timeout)
	n := reqNum.Add(1)
	latency, result := invokeTarget(reqCtx, target, CallData{RequestNumber: n, WorkerID: workerID})
	ctxEnded := reqCtx.Err() != nil
	cancel()
	if truncated && ctxEnded && isDeadlineOrCancel(result.TransportErr) && !time.Now().Before(windowEnd) {
		dropped.Add(1)
		return
	}
	samples <- sample{sentAt: sentAt, latency: latency, result: result}
}

// isDeadlineOrCancel reports whether err is a context deadline/cancellation
// outcome, either as a wrapped context error (in-process targets returning
// ctx.Err()) or as the equivalent gRPC status code (real transport calls).
func isDeadlineOrCancel(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	code := status.Code(err)
	return code == codes.DeadlineExceeded || code == codes.Canceled
}

// invokeTarget executes target and recovers panics as ErrTargetPanic transport errors.
func invokeTarget(ctx context.Context, target ResultTarget, data CallData) (latency time.Duration, result TargetResult) {
	start := time.Now()
	defer func() {
		latency = time.Since(start)
		if v := recover(); v != nil {
			result = TargetResult{TransportErr: ErrTargetPanic{Value: v, Stack: string(debug.Stack())}}
		}
	}()
	result = target(ctx, data)
	return
}

// errorCode returns the canonical gRPC status code name for err. Errors that
// aren't gRPC statuses map to "UNKNOWN".
func errorCode(err error) string {
	if err == nil {
		return ""
	}
	return status.Code(err).String()
}

// usToMs converts HDR histogram microsecond values to milliseconds for wire output.
func usToMs(us int64) float64 {
	return float64(us) / 1000.0
}
