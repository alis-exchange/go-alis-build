package loadgen

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
	"go.alis.build/alog"
	"google.golang.org/grpc/status"
)

// Generator runs one load window against a target function. Implementations
// must be safe to call sequentially against different profiles.
type Generator interface {
	Run(ctx context.Context, p Profile, target ResultTarget) (*Metrics, error)
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

// hdrConfig covers 1µs to 1h with 3 significant figures. Latencies above 1h
// are extremely unlikely for a single RPC and would be clamped to the max.
const (
	hdrMinValueUs = 1                                      // minimum recordable latency in microseconds
	hdrMaxValueUs = int64(time.Hour / time.Microsecond)    // maximum recordable latency in microseconds
	hdrSigFigs    = 3                                      // HDR histogram significant-figure precision
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
// pool paced by the profile's selected pacer.
//
// Transport failures increment Metrics.ErrorCount and ErrorsByCode; semantic
// check failures increment CheckFailedCount. Neither aborts the window. Only
// ctx cancellation, abort-on-SLO, and invalid profile produce a returned
// error (with partial metrics for cancellation/abort).
func (g *inProcess) Run(ctx context.Context, p Profile, target ResultTarget) (*Metrics, error) {
	if target == nil {
		return nil, ErrNilTarget{}
	}
	if err := p.validate(); err != nil {
		return nil, err
	}

	total := p.Warmup + p.Duration
	pacer := pacerForProfile(p, total)
	maxWorkers := p.MaxConcurrency()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	ticks := make(chan time.Time, maxWorkers)
	samples := make(chan sample, maxWorkers*4)
	var dropped atomic.Int64
	var inFlight atomic.Int32

	start := time.Now()
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
			runWorker(workerCtx, ticks, samples, target, reqTimeout, total, &inFlight, &reqNum, &dropped, id)
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
	go func() {
		defer close(pacerDone)
		defer close(ticks)
		var sent uint64
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			elapsed := time.Since(start)
			wait, stop := pacer.Pace(elapsed, sent)
			if stop {
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
	}()

	<-pacerDone

	if p.GracefulRampDown > 0 {
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(p.GracefulRampDown):
			workerMu.Lock()
			for _, cancel := range stops {
				cancel()
			}
			workerMu.Unlock()
			wg.Wait()
		}
	} else {
		wg.Wait()
	}

	close(samples)
	m := <-aggDone
	m.DroppedCount = dropped.Load()

	if ctxErr := runCtx.Err(); ctxErr != nil {
		return m, ctxErr
	}

	targetQPS := p.EffectiveQPS()
	if m.ActualQPS > 0 && m.ActualQPS < saturationThreshold*targetQPS {
		alog.Warnf(runCtx, "loadgen saturated: actual %.1f qps < target %.1f qps (concurrency=%d, mean latency %.1fms)",
			m.ActualQPS, targetQPS, p.MaxConcurrency(), m.Latency.MeanMs)
	}
	return m, nil
}

// runAbortWatcher polls AbortCheck every 2s and cancels the run when it returns true.
func runAbortWatcher(ctx context.Context, cancel context.CancelFunc, check AbortCheck, agg *aggregator) {
	ticker := time.NewTicker(2 * time.Second)
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
	m := &Metrics{
		Duration:          a.measureFor,
		RequestCount:      a.count,
		ErrorCount:        a.errCount,
		CheckPassedCount:  a.checkPassed,
		CheckFailedCount:  a.checkFailed,
		ErrorsByCode:      cloneErrorsMap(a.errorsByCode),
		DroppedCount:      a.dropped,
		MeasurementStart:  a.measurementStart,
		MeasurementEnd:    a.measurementEnd,
	}
	if a.measureFor > 0 {
		m.ActualQPS = float64(a.count) / a.measureFor.Seconds()
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
	m := &Metrics{
		Duration:     a.measureFor,
		RequestCount: a.count,
		ErrorCount:   a.errCount,
	}
	if a.measureFor > 0 {
		m.ActualQPS = float64(a.count) / a.measureFor.Seconds()
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

// runWorker pulls ticks and executes the target with a per-request timeout.
// Panics in the target are recovered and recorded as INTERNAL errors so the
// window keeps running.
func runWorker(parent context.Context, ticks <-chan time.Time, samples chan<- sample, target ResultTarget, reqTimeout, windowTotal time.Duration, inFlight *atomic.Int32, reqNum *atomic.Uint64, dropped *atomic.Int64, workerID int) {
	for sentAt := range ticks {
		if err := parent.Err(); err != nil {
			inFlight.Add(-1)
			return
		}
		remaining := windowTotal - time.Since(sentAt)
		if remaining <= 0 {
			// Ticks scheduled before the boundary but executed after it are excluded
			// from aggregates; count as dropped so ActualQPS stays honest under ramp-down.
			dropped.Add(1)
			inFlight.Add(-1)
			continue
		}
		timeout := reqTimeout
		if remaining < timeout {
			timeout = remaining
		}
		reqCtx, cancel := context.WithTimeout(parent, timeout)
		n := reqNum.Add(1)
		latency, result := invokeTarget(reqCtx, target, CallData{RequestNumber: n, WorkerID: workerID})
		cancel()
		samples <- sample{sentAt: sentAt, latency: latency, result: result}
		inFlight.Add(-1)
	}
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
