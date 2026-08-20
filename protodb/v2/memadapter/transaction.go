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
// irrelevant for its purpose — correctness (no torn reads, and — via
// liveTables below — real rollback-on-error) is what matters, and one
// mutex gives that for free.
var mu sync.Mutex

// tableHooks lets RunTransaction snapshot and restore one Table[R]'s
// entries without knowing R at the call site: R varies per table, so a
// single package-global registry can't hold a concretely-typed
// []*Table[R] — instead each hook pair closes over its own Table[R] and
// erases R behind `any`.
type tableHooks struct {
	// snapshot returns a copy of the table's current entries map, boxed as
	// any.
	snapshot func() any
	// restore replaces the table's entries map with a value previously
	// returned by snapshot.
	restore func(any)
}

// liveTables is the package-global registry of snapshot/restore hooks for
// every Table[R] ever created by New, across every instantiation of R.
// registerTable appends to it; RunTransaction reads it to snapshot every
// live table before running fn and to restore them if fn errors.
//
// The registry is append-only — entries are never removed — so it grows
// for the life of the process. That mirrors memadapter's role as a test
// double (see the package doc): a long-running production process that
// created unbounded Table instances would leak, but no consumer of
// memadapter does that, and the simplicity is worth it here.
var liveTables []tableHooks

// registerTable adds t's snapshot/restore hooks to liveTables. Called once
// per table, from New.
func registerTable[R any](t *Table[R]) {
	mu.Lock()
	defer mu.Unlock()
	liveTables = append(liveTables, tableHooks{
		snapshot: func() any {
			// entries is never mutated in place — every write replaces a
			// map slot with a new *entry rather than editing one's fields
			// (see the entry doc comment) — so a shallow copy of the map
			// is a fully consistent point-in-time snapshot: even if t
			// keeps writing after snapshot returns, the copied map's
			// slots still point at the *entry values as they were at
			// snapshot time.
			cp := make(map[string]*entry[R], len(t.entries))
			for k, v := range t.entries {
				cp[k] = v
			}
			return cp
		},
		restore: func(s any) {
			t.entries = s.(map[string]*entry[R])
		},
	})
}

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
// real atomicity across memadapter tables: it holds the single
// package-global mutex (mu) for fn's entire execution — while
// RunTransaction holds it, no other goroutine's table operation (List,
// Read, Write, Stream, ...) on ANY memadapter table can interleave — and it
// rolls back on error. Before invoking fn, RunTransaction snapshots every
// live table's entries; if fn returns a non-nil error, every table's
// entries are restored to that snapshot before RunTransaction returns, so
// none of fn's writes are observable afterward. This matches
// protodb.TransactionRunner's documented rollback-on-error contract.
//
// Unlike spanneradapter.SpannerTransactionRunner, fn runs exactly once —
// there is no abort/retry simulation, because there is no transient
// conflict here that would call for a retry (mu already serializes every
// transaction). Consumers that rely on protodb.ReadModifyWrite's "fn may
// run more than once" contract are still safe: running once is a valid
// (degenerate) case of "may run more than once".
func NewTransactionRunner() protodb.TransactionRunner {
	return transactionRunner{}
}

// RunTransaction implements protodb.TransactionRunner. It commits fn's
// writes by simply leaving them in place when fn returns nil, and rolls
// them back by restoring every live table's pre-fn snapshot when fn
// returns a non-nil error — see NewTransactionRunner.
//
// Nested calls are unsupported and WILL DEADLOCK: fn must not call
// RunTransaction again (on this or any other memadapter runner) with the
// ctx it was given, or with a derivative of it — mu is already held for
// the outer call, sync.Mutex is not reentrant, and the inner RunTransaction
// would block forever trying to lock it. Table operations are fine to
// nest (that's the whole point of the ctx marker); RunTransaction itself
// is not.
func (transactionRunner) RunTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	mu.Lock()
	defer mu.Unlock()

	snapshots := make([]any, len(liveTables))
	for i, h := range liveTables {
		snapshots[i] = h.snapshot()
	}

	err := fn(context.WithValue(ctx, transactionMarkerKey{}, true))
	if err != nil {
		for i, h := range liveTables {
			h.restore(snapshots[i])
		}
	}
	return err
}
