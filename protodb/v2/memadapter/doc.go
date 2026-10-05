// Package memadapter provides an in-memory implementation of
// protodb.ResourceTable, used as a test double throughout the protodb
// ecosystem and as the second adapter (alongside spanneradapter) proving
// the ResourceTable seam.
//
// # Storage model
//
// A Table[R] holds its rows in a plain Go map, keyed by the canonical JSON
// encoding of the row's Key.KeyValues() (mirroring
// spanneradapter's KeySpec.Encode — meaningful only for equality
// comparison within one process, never decoded back). Each stored entry is
// created fresh and never mutated in place — Write and WritePolicies
// replace the map slot with a new *entry rather than editing fields of an
// existing one — so once a *entry is handed to a reader it is safe to keep
// using after the lock is released.
//
// R is deep-copied on the way in (Create/Write) and out (Read/BatchRead/
// List/Stream) whenever it implements proto.Message, via proto.Clone
// behind a type switch: this protects the store from a caller mutating the
// Row they passed to Create/Write after the call returns, and protects the
// caller from mutating memadapter's internal state through a Row a read
// returned. R's IAM Policy (*iampb.Policy) is cloned the same way.
//
// Caveat: for R that is NOT a proto.Message — including plain structs,
// non-proto pointer types, maps, and slices — memadapter stores and
// returns the value as-is (plain assignment). If such an R holds pointers,
// the caller and memadapter's internal state alias each other; mutating
// one mutates the other. Use a proto.Message resource type (the common
// case throughout protodb) to get memadapter's copy-on-write/copy-on-read
// isolation.
//
// # Locking and transactions
//
// See transaction.go: every Table[R] and every runner returned by
// NewTransactionRunner shares one package-global sync.Mutex. RunTransaction
// gives real rollback-on-error: it snapshots every live table's entries map
// before running fn and restores those snapshots if fn returns a non-nil
// error, so a failed transaction leaves every table exactly as it was
// before the transaction started, matching protodb.TransactionRunner's
// documented contract.
package memadapter
