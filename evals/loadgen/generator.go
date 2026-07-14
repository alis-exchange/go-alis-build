package loadgen

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
	"go.alis.build/alog"
	"google.golang.org/grpc/status"
)

// Target executes exactly one request. A non-nil return marks the request as
// an error and increments ErrorsByCode[status.Code(err).String()].
type Target func(context.Context) error

// Generator runs one load window against a target function. Implementations
// must be safe to call sequentially against different profiles.
type Generator interface {
	Run(ctx context.Context, p Profile, target Target) (*Metrics, error)
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
	err     error
}

// Run executes the load window described by p, driving target via a pool of
// Concurrency workers paced by a ConstantPacer.
//
// Target errors are data — they land in Metrics.ErrorsByCode and ErrorCount
// and never abort the window. Only ctx cancellation and invalid profile
// produce a returned error (with partial metrics for ctx cancellation).
func (g *inProcess) Run(ctx context.Context, p Profile, target Target) (*Metrics, error) {
	if target == nil {
		return nil, ErrNilTarget{}
	}
	if err := p.validate(); err != nil {
		return nil, err
	}

	total := p.Warmup + p.Duration
	pacer := ConstantPacer{Freq: p.QPS, Duration: total}

	// Channels
	ticks := make(chan time.Time, p.Concurrency)
	samples := make(chan sample, p.Concurrency*4)

	// Aggregator: single owner of the histogram + counters, no locks.
	aggDone := make(chan *Metrics, 1)
	measurementStart := time.Time{} // set after ticks begin (start + Warmup)
	go func() {
		aggDone <- aggregate(samples, &measurementStart, p.Duration)
	}()

	// Workers: fixed pool consuming ticks.
	reqTimeout := p.resolvedRequestTimeout()
	var wg sync.WaitGroup
	wg.Add(p.Concurrency)
	for i := 0; i < p.Concurrency; i++ {
		go func() {
			defer wg.Done()
			runWorker(ctx, ticks, samples, target, reqTimeout, total)
		}()
	}

	// Pacer: absolute-offset scheduling relative to `start`.
	start := time.Now()
	measurementStart = start.Add(p.Warmup)
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
				case <-ctx.Done():
					return
				}
			}
			now := time.Now()
			select {
			case ticks <- now:
				sent++
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for pacer to stop (window ended or ctx cancelled), then drain
	// workers, then wait for the aggregator.
	<-pacerDone
	wg.Wait()
	close(samples)
	m := <-aggDone

	if ctxErr := ctx.Err(); ctxErr != nil {
		return m, ctxErr
	}

	// Saturation warning: if we couldn't hold the target rate, surface it.
	if m.ActualQPS > 0 && m.ActualQPS < saturationThreshold*p.QPS {
		alog.Warnf(ctx, "loadgen saturated: actual %.1f qps < target %.1f qps (concurrency=%d, mean latency %.1fms)",
			m.ActualQPS, p.QPS, p.Concurrency, m.Latency.MeanMs)
	}
	return m, nil
}

// runWorker pulls ticks and executes the target with a per-request timeout.
// Panics in the target are recovered and recorded as INTERNAL errors so the
// window keeps running.
func runWorker(parent context.Context, ticks <-chan time.Time, samples chan<- sample, target Target, reqTimeout, windowTotal time.Duration) {
	for sentAt := range ticks {
		// Cap the per-request timeout by the remaining window; a request
		// that outlives the window pollutes the next case.
		remaining := windowTotal - time.Since(sentAt)
		if remaining <= 0 {
			continue
		}
		timeout := reqTimeout
		if remaining < timeout {
			timeout = remaining
		}
		reqCtx, cancel := context.WithTimeout(parent, timeout)
		latency, err := invokeTarget(reqCtx, target)
		cancel()
		samples <- sample{sentAt: sentAt, latency: latency, err: err}
	}
}

// invokeTarget runs the target under a panic-recovering defer. A panic is
// converted to an error so aggregation counts it as a failed request.
func invokeTarget(ctx context.Context, target Target) (latency time.Duration, err error) {
	start := time.Now()
	defer func() {
		latency = time.Since(start)
		if v := recover(); v != nil {
			err = ErrTargetPanic{Value: v, Stack: string(debug.Stack())}
		}
	}()
	err = target(ctx)
	return
}

// aggregate drains samples into an HDR histogram and counters. Only samples
// with sentAt >= *measurementStart are counted, so the warmup window is
// naturally excluded.
func aggregate(samples <-chan sample, measurementStart *time.Time, measureFor time.Duration) *Metrics {
	hist := hdrhistogram.New(hdrMinValueUs, hdrMaxValueUs, hdrSigFigs)
	errorsByCode := map[string]int64{}
	var count, errCount int64
	var latencySumUs float64

	for s := range samples {
		if measurementStart.IsZero() || s.sentAt.Before(*measurementStart) {
			continue
		}
		count++
		us := s.latency.Microseconds()
		if us < hdrMinValueUs {
			us = hdrMinValueUs
		}
		if us > hdrMaxValueUs {
			us = hdrMaxValueUs
		}
		_ = hist.RecordValue(us)
		latencySumUs += float64(s.latency) / float64(time.Microsecond)
		if s.err != nil {
			errCount++
			errorsByCode[errorCode(s.err)]++
		}
	}

	m := &Metrics{
		Duration:     measureFor,
		RequestCount: count,
		ErrorCount:   errCount,
		ErrorsByCode: errorsByCode,
	}
	if measureFor > 0 {
		m.ActualQPS = float64(count) / measureFor.Seconds()
	}
	if count > 0 {
		m.Latency = LatencySummary{
			P50Ms:  usToMs(hist.ValueAtQuantile(50)),
			P95Ms:  usToMs(hist.ValueAtQuantile(95)),
			P99Ms:  usToMs(hist.ValueAtQuantile(99)),
			MinMs:  usToMs(hist.Min()),
			MaxMs:  usToMs(hist.Max()),
			MeanMs: (latencySumUs / float64(count)) / 1000.0,
		}
	}
	return m
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
