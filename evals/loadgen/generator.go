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

type inProcess struct{}

// hdrConfig covers 1µs to 1h with 3 significant figures. Latencies above 1h
// are extremely unlikely for a single RPC and would be clamped to the max.
const (
	hdrMinValueUs = 1
	hdrMaxValueUs = int64(time.Hour / time.Microsecond)
	hdrSigFigs    = 3
)

type sample struct {
	sentAt  time.Time
	latency time.Duration
	result  TargetResult
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
				dropped.Add(1)
			} else {
				select {
				case ticks <- now:
					sent++
					inFlight.Add(1)
				default:
					dropped.Add(1)
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

type aggregator struct {
	measurementStart time.Time
	measurementEnd   time.Time
	measureFor       time.Duration
	dropped          int64

	mu            sync.Mutex
	hist          *hdrhistogram.Histogram
	ttfbHist      *hdrhistogram.Histogram
	respHist      *hdrhistogram.Histogram
	totalHist     *hdrhistogram.Histogram
	errorsByCode  map[string]int64
	count         int64
	errCount      int64
	checkPassed   int64
	checkFailed   int64
	latencySumUs  float64
	streamCount   int64
	messagesTotal int64
}

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

func (a *aggregator) finalize() *Metrics {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.buildMetrics()
}

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

func (a *aggregator) buildMetrics() *Metrics {
	m := &Metrics{
		Duration:         a.measureFor,
		RequestCount:     a.count,
		ErrorCount:       a.errCount,
		CheckPassedCount: a.checkPassed,
		CheckFailedCount: a.checkFailed,
		ErrorsByCode:     cloneErrorsMap(a.errorsByCode),
		DroppedCount:     a.dropped,
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

func usToMs(us int64) float64 {
	return float64(us) / 1000.0
}
