package loadgen

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// workFunc is the body of one worker. It returns when stop closes or when
// its own exit condition holds (the tick channel closed, or the window
// ended). stop only means "take no new work": a call already running
// finishes under callsCtx, which the pool cancels only at the ramp-down
// cutoff or when the context it was built with is cancelled. The pool
// closes stop on a scale-down and at the ramp-down cutoff, never at the
// start of a graceful drain.
type workFunc func(callsCtx context.Context, stop <-chan struct{}, id int)

// workerPool owns the worker goroutines of one load window. It starts them,
// resizes them to follow ConcurrencyStages, and drains them when the window
// closes. Both dispatch modes plug in as a workFunc.
type workerPool struct {
	// callsCtx bounds every call a worker makes. It derives from the
	// context passed to newWorkerPool, so abort and caller cancellation
	// reach in-flight calls.
	callsCtx context.Context
	// cancelCalls cuts in-flight calls at the ramp-down cutoff.
	cancelCalls context.CancelFunc
	// work is the body every worker runs.
	work workFunc

	// wg counts running worker goroutines. Add is only called by Start and
	// the supervisor, and Drain stops the supervisor before Wait.
	wg sync.WaitGroup
	// mu guards stops and nextID.
	mu sync.Mutex
	// stops holds the stop channel of each live worker, oldest first.
	stops []chan struct{}
	// nextID is the highest worker ID reserved so far. Start reserves
	// 0..initial (initial itself is never used); supervisor-added workers
	// take nextID+1.
	nextID int
	// live mirrors len(stops) so Size stays lock-free for the pacer.
	live atomic.Int32

	// cancelSupervisor stops the concurrency supervisor; nil without stages.
	cancelSupervisor context.CancelFunc
	// supervisorDone closes when the supervisor goroutine has returned.
	supervisorDone chan struct{}
}

// newWorkerPool returns an empty pool whose calls run under a context
// derived from callsCtx.
func newWorkerPool(callsCtx context.Context, work workFunc) *workerPool {
	ctx, cancel := context.WithCancel(callsCtx)
	return &workerPool{callsCtx: ctx, cancelCalls: cancel, work: work}
}

// Start launches initial workers with IDs 0..initial-1. When stages is
// non-empty it also starts the concurrency supervisor, which resizes the
// pool against elapsed time since start.
func (p *workerPool) Start(initial int, stages []Stage, start time.Time) {
	p.mu.Lock()
	for id := range initial {
		p.spawnLocked(id)
	}
	// Supervisor-added workers keep the numbering Run used before the pool
	// existed: the first one gets initial+1.
	p.nextID = initial
	p.mu.Unlock()
	if len(stages) == 0 {
		return
	}
	supCtx, cancel := context.WithCancel(p.callsCtx)
	p.cancelSupervisor = cancel
	p.supervisorDone = make(chan struct{})
	go func() {
		defer close(p.supervisorDone)
		runConcurrencySupervisor(supCtx, start, stages, p.add, p.remove, p.Size)
	}()
}

// Size returns the number of live workers. Workers told to stop but still
// finishing a call are not counted.
func (p *workerPool) Size() int { return int(p.live.Load()) }

// add starts one more worker.
func (p *workerPool) add() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextID++
	p.spawnLocked(p.nextID)
}

// remove tells the newest live worker to stop taking new work. Size drops
// before stop closes, so a worker that sees stop also sees the new size.
func (p *workerPool) remove() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.stops) == 0 {
		return
	}
	last := len(p.stops) - 1
	stop := p.stops[last]
	p.stops = p.stops[:last]
	p.live.Add(-1)
	close(stop)
}

// stopAll closes the stop channel of every worker still held and sets
// Size to 0. Only the ramp-down cutoff in Drain calls it.
func (p *workerPool) stopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.live.Store(0)
	for _, stop := range p.stops {
		close(stop)
	}
	p.stops = nil
}

// spawnLocked starts one worker with the given ID. p.mu must be held.
func (p *workerPool) spawnLocked(id int) {
	stop := make(chan struct{})
	p.stops = append(p.stops, stop)
	p.live.Add(1)
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.work(p.callsCtx, stop, id)
	}()
}

// Drain shuts the pool down. It stops the supervisor and waits for it to
// return, so no worker can be added afterwards. It then waits up to
// rampDown for every worker to return on its own, without closing any
// stop channel: open-loop workers drain the closed tick channel and
// closed-loop workers stop at the window end. At the cutoff (rampDown
// elapsed, rampDown of zero or less, or the calls context cancelled during
// the wait) it closes every remaining stop channel, cancels the calls
// context to cut in-flight calls, and waits for every worker to return.
func (p *workerPool) Drain(rampDown time.Duration) {
	if p.cancelSupervisor != nil {
		p.cancelSupervisor()
		<-p.supervisorDone
	}
	// The supervisor has returned, so no Add can race this Wait.
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	if rampDown > 0 {
		timer := time.NewTimer(rampDown)
		select {
		case <-done:
		case <-timer.C:
		case <-p.callsCtx.Done():
		}
		timer.Stop()
	}
	// Cutoff. Closing the stop of a worker that already returned is
	// harmless; scale-down already removed the channels it closed.
	p.stopAll()
	p.cancelCalls()
	<-done
}
