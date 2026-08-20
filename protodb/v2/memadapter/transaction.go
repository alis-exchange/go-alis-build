package memadapter

import (
	"context"
	"sync"

	"go.alis.build/protodb/v2"
)

// mu is THE single lock behind every memadapter Table and every runner
// returned by NewTransactionRunner. There is no per-table locking: all
// memadapter activity in a process — reads, writes, lists, streams, and
// transactions, across every Table[R] instance — serializes on this one
// mutex.
//
// That is a deliberate simplification, not an oversight: memadapter is a
// test double and the second (in-memory) proof of the ResourceTable seam,
// not a production store. Contention under a single global lock is
// irrelevant for its purpose — correctness (atomicity, no torn reads) is
// what matters, and one mutex gives that for free without per-table
// bookkeeping.
var mu sync.Mutex

// transactionMarkerKey is the context key RunTransaction sets so that
// table operations invoked with its ctx can tell they are already running
// inside the locked transaction.
type transactionMarkerKey struct{}

// lock acquires mu for the duration of one table operation, UNLESS ctx is
// already running inside a RunTransaction call (detected via
// transactionMarkerKey) — in which case it is a no-op. This is what makes
// the model self-consistent: sync.Mutex is not reentrant, so a table op
// called from inside RunTransaction's fn must not try to lock mu again (it
// would deadlock against the very lock RunTransaction is holding around
// fn). Outside a transaction, every table op takes and releases mu on its
// own, one call at a time.
//
// The returned function releases whatever lock() actually took, which may
// be nothing; callers always call it, typically via `defer lock(ctx)()`.
func lock(ctx context.Context) (unlock func()) {
	if ctx.Value(transactionMarkerKey{}) != nil {
		return func() {}
	}
	mu.Lock()
	return mu.Unlock
}

// transactionRunner is the protodb.TransactionRunner backing
// NewTransactionRunner. It carries no state of its own — mu is the shared
// package-global lock every Table already uses.
type transactionRunner struct{}

// NewTransactionRunner returns a protodb.TransactionRunner that provides
// atomicity across memadapter tables by holding the single package-global
// mutex (mu) for fn's entire execution: while RunTransaction holds it, no
// other goroutine's table operation (List, Read, Write, Stream, ...) on ANY
// memadapter table can interleave.
//
// Unlike spanneradapter.SpannerTransactionRunner, fn runs exactly once —
// there is no abort/retry simulation, because there is nothing here that
// can abort. Consumers that rely on protodb.ReadModifyWrite's "fn may run
// more than once" contract are still safe: running once is a valid
// (degenerate) case of "may run more than once".
func NewTransactionRunner() protodb.TransactionRunner {
	return transactionRunner{}
}

// RunTransaction implements protodb.TransactionRunner.
func (transactionRunner) RunTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	mu.Lock()
	defer mu.Unlock()
	return fn(context.WithValue(ctx, transactionMarkerKey{}, true))
}
