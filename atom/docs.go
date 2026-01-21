/*
Package atom implements the Transaction/Unit of Work pattern with compensating
actions for ensuring atomicity in non-database operations like network requests,
file I/O, and other side effects.

# Overview

Atom provides a way to group multiple operations into a single unit of work that
can be rolled back if any operation fails. Each operation is paired with a
compensating function that undoes its effects.

# Basic Usage

	tx := atom.NewTransaction()
	defer func() {
		if !tx.IsCommitted() {
			_ = tx.Rollback(ctx)
		}
	}()

	err := tx.Do(ctx, "create-file",
		func(ctx context.Context) error {
			return os.WriteFile("/tmp/file.txt", []byte("data"), 0644)
		},
		func(ctx context.Context) error {
			return os.Remove("/tmp/file.txt")
		},
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)

# Operations with Options

Use DoWithOptions for advanced configuration including timeouts and retry:

	err := tx.DoWithOptions(ctx, "api-call", atom.OperationOptions{
		Timeout: 5 * time.Second,
		CompensationRetry: atom.DefaultRetryOptions(),
	}, operationFunc, compensateFunc)

# Savepoints

Create savepoints for partial rollback:

	sp := tx.CreateSavepoint("checkpoint")
	// ... more operations ...
	tx.RollbackToSavepoint(ctx, sp) // rolls back only operations after savepoint

# Hooks

Register callbacks at various points in the transaction lifecycle:

	tx.AddHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		// validation logic
		return nil
	})

Hook types: BeforeCommit, AfterCommit, BeforeRollback, AfterRollback,
BeforeOperation, AfterOperation.

# Observability

Implement the Observer interface for metrics and tracing:

	tx.SetObserver(&MyObserver{})

# Logging

Optionally configure a *slog.Logger for structured logging:

	tx.SetLogger(slog.Default())

By default, no logging is performed. When a logger is set, warnings and info
messages are logged for rollback failures, hook failures, and retry attempts.

# Error Types

The errors subpackage provides typed errors:

  - OperationError: wraps errors from operation execution
  - RollbackError: wraps errors from compensation (supports multi-error unwrap)
  - HookError: wraps errors from hook execution
  - PanicError: wraps recovered panics

# Thread Safety

All Transaction methods are safe for concurrent use. The implementation uses
RWMutex and atomic operations internally.
*/
package atom // import "go.alis.build/atom"
