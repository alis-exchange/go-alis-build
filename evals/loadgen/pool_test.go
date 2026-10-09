package loadgen

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// TestWorkerPool_StopDoesNotCancelCalls pins that a scale-down stop signal
// only stops a worker taking new work: the calls context is still live
// when stop closes.
func TestWorkerPool_StopDoesNotCancelCalls(t *testing.T) {
	t.Parallel()

	errs := make(chan error, 2)
	pool := newWorkerPool(context.Background(), func(callsCtx context.Context, stop <-chan struct{}, _ int) {
		<-stop
		errs <- callsCtx.Err()
	})
	pool.Start(2, []Stage{
		{Duration: 10 * time.Millisecond, Target: 2},
		{Duration: time.Second, Target: 1},
	}, time.Now())
	if got := pool.Size(); got != 2 {
		t.Fatalf("Size()=%d after Start(2), want 2", got)
	}
	var err error
	select {
	case err = <-errs:
	case <-time.After(2 * time.Second):
		t.Fatal("no worker stopped within 2s of the step down to 1")
	}
	if err != nil {
		t.Fatalf("callsCtx.Err() when the scale-down closed stop = %v, want nil", err)
	}
	// remove decrements Size before it closes stop, so this read is
	// ordered after the decrement.
	if got := pool.Size(); got != 1 {
		t.Fatalf("Size()=%d after the step down, want 1", got)
	}
	pool.Drain(0)
	if got := pool.Size(); got != 0 {
		t.Fatalf("Size()=%d after Drain, want 0", got)
	}
}

// TestWorkerPool_GracefulDrainDoesNotStopWorkers pins the graceful end:
// Drain waits for workers to finish on their own and closes no stop
// channel before the ramp-down cutoff, so queued work is not stranded.
func TestWorkerPool_GracefulDrainDoesNotStopWorkers(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	var sawStop atomic.Int32
	pool := newWorkerPool(context.Background(), func(_ context.Context, stop <-chan struct{}, _ int) {
		select {
		case <-stop:
			sawStop.Add(1)
		case <-release:
		}
	})
	pool.Start(2, nil, time.Now())
	time.AfterFunc(20*time.Millisecond, func() { close(release) })
	begin := time.Now()
	pool.Drain(time.Second)
	if elapsed := time.Since(begin); elapsed > 500*time.Millisecond {
		t.Fatalf("Drain(1s) took %v, want < 500ms: it waited for the cutoff instead of the workers", elapsed)
	}
	if got := sawStop.Load(); got != 0 {
		t.Fatalf("%d workers saw stop closed, want 0: a graceful Drain must not stop workers", got)
	}
	if got := pool.Size(); got != 0 {
		t.Fatalf("Size()=%d after Drain, want 0", got)
	}
}

// TestWorkerPool_DrainCutsCallsAfterRampDown pins the ramp-down cutoff: a
// call that ignores stop is cut by cancelling the calls context once the
// ramp-down has elapsed.
func TestWorkerPool_DrainCutsCallsAfterRampDown(t *testing.T) {
	t.Parallel()

	pool := newWorkerPool(context.Background(), func(callsCtx context.Context, _ <-chan struct{}, _ int) {
		<-callsCtx.Done()
	})
	pool.Start(1, nil, time.Now())
	begin := time.Now()
	pool.Drain(30 * time.Millisecond)
	elapsed := time.Since(begin)
	if elapsed < 30*time.Millisecond || elapsed > time.Second {
		t.Fatalf("Drain(30ms) took %v, want between 30ms and 1s", elapsed)
	}
}

// TestWorkerPool_CallerCancelReachesCalls pins that the calls context
// derives from the context the pool was built with.
func TestWorkerPool_CallerCancelReachesCalls(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithCancel(context.Background())
	ended := make(chan struct{})
	pool := newWorkerPool(parent, func(callsCtx context.Context, _ <-chan struct{}, _ int) {
		<-callsCtx.Done()
		close(ended)
	})
	pool.Start(1, nil, time.Now())
	cancel()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("worker still running 1s after the parent context was cancelled")
	}
	pool.Drain(0)
}

// TestWorkerPool_ScaleDownStopsNewestWorkersFirst pins supervisor-driven
// scale-down: the newest workers are stopped and Size reports live
// workers only.
func TestWorkerPool_ScaleDownStopsNewestWorkersFirst(t *testing.T) {
	t.Parallel()

	stopped := make(chan int, 3)
	pool := newWorkerPool(context.Background(), func(_ context.Context, stop <-chan struct{}, id int) {
		<-stop
		stopped <- id
	})
	pool.Start(3, []Stage{
		{Duration: 10 * time.Millisecond, Target: 3},
		{Duration: time.Second, Target: 1},
	}, time.Now())

	deadline := time.Now().Add(2 * time.Second)
	for pool.Size() != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("Size()=%d after 2s, want 1", pool.Size())
		}
		time.Sleep(5 * time.Millisecond)
	}
	got := []int{<-stopped, <-stopped}
	slices.Sort(got)
	if want := []int{1, 2}; !slices.Equal(got, want) {
		t.Fatalf("stopped worker IDs=%v, want %v", got, want)
	}
	// Drain(0) goes straight to the cutoff, which closes the last stop.
	pool.Drain(0)
	if id := <-stopped; id != 0 {
		t.Fatalf("Drain stopped worker %d, want 0", id)
	}
}

// TestWorkerPool_DrainStopsSupervisorFirst pins the drain order. The first
// supervisor tick (~50ms) would see the four-worker stage, but Drain runs
// before it and must stop the supervisor, so no worker is ever added.
func TestWorkerPool_DrainStopsSupervisorFirst(t *testing.T) {
	t.Parallel()

	var started atomic.Int32
	pool := newWorkerPool(context.Background(), func(_ context.Context, stop <-chan struct{}, _ int) {
		started.Add(1)
		<-stop
	})
	pool.Start(1, []Stage{
		{Duration: time.Millisecond, Target: 1},
		{Duration: time.Millisecond, Target: 4},
	}, time.Now())
	// Drain(0): the worker only returns on stop, which a graceful Drain
	// would hold until the cutoff.
	pool.Drain(0)
	time.Sleep(100 * time.Millisecond) // two supervisor ticks
	if got := started.Load(); got != 1 {
		t.Fatalf("workers started=%d, want 1: the supervisor added workers after Drain", got)
	}
	if got := pool.Size(); got != 0 {
		t.Fatalf("Size()=%d after Drain, want 0", got)
	}
}
