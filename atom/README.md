# Atom

Atom implements the Transaction/Unit of Work pattern with compensating actions for ensuring atomicity in non-database operations like network requests, file I/O, and other side effects.

## Features

- **Automatic Rollback**: Failed operations trigger automatic rollback of all previous operations in reverse order (LIFO)
- **Panic Recovery**: Panics in operations or compensating functions are caught and handled gracefully
- **Hooks System**: Add callbacks at key points (BeforeCommit, AfterCommit, BeforeRollback, AfterRollback, BeforeOperation, AfterOperation)
- **Critical Hooks**: Hooks can block operations (e.g., validation before commit)
- **Operation Metadata**: Track execution time, errors, and operation history
- **Thread-Safe**: Concurrent access is protected with RWMutex
- **Context Support**: Full context propagation for cancellation and timeouts
- **Savepoints**: Create savepoints for partial rollback
- **Operation Timeouts**: Configure per-operation timeouts
- **Compensation Retry**: Automatic retry with exponential backoff for failed compensations
- **Observability**: Observer interface for metrics and tracing integration

## Installation

```bash
go get go.alis.build/atom
```

## Basic Usage

```go
ctx := context.Background()
tx := atom.NewTransaction()

// Always ensure cleanup
defer func() {
    if !tx.IsCommitted() {
        _ = tx.Rollback(ctx)
    }
}()

// Execute operations with compensating functions
err := tx.Do(ctx, "create-file", 
    func(ctx context.Context) error {
        return os.WriteFile("/tmp/file.txt", []byte("data"), 0644)
    },
    func(ctx context.Context) error {
        return os.Remove("/tmp/file.txt")
    },
)
if err != nil {
    return err // Rollback happens automatically via defer
}

// More operations...

// Commit to finalize
return tx.Commit(ctx)
```

## Operations with Options

Use `DoWithOptions` for advanced operation configuration:

```go
err := tx.DoWithOptions(ctx, "api-call", atom.OperationOptions{
    Timeout: 5 * time.Second,
    CompensationRetry: &atom.RetryOptions{
        MaxRetries:        3,
        InitialDelay:      100 * time.Millisecond,
        BackoffMultiplier: 2.0,
        MaxDelay:          5 * time.Second,
    },
}, 
    func(ctx context.Context) error {
        return callExternalAPI(ctx)
    },
    func(ctx context.Context) error {
        return rollbackAPICall(ctx)
    },
)
```

## Savepoints

Create savepoints for partial rollback:

```go
tx := atom.NewTransaction()

// First batch of operations
_ = tx.Do(ctx, "op1", op1Func, comp1Func)
_ = tx.Do(ctx, "op2", op2Func, comp2Func)

// Create a savepoint
sp := tx.CreateSavepoint("after-batch-1")

// Second batch of operations
_ = tx.Do(ctx, "op3", op3Func, comp3Func)
_ = tx.Do(ctx, "op4", op4Func, comp4Func)

// Rollback only the second batch
if someCondition {
    err := tx.RollbackToSavepoint(ctx, sp)
    // op3 and op4 are rolled back, op1 and op2 remain
}

// Continue with more operations or commit
_ = tx.Commit(ctx)
```

## Hooks

### Hook Types

- **BeforeCommit**: Called before commit (critical by default - failures block commit)
- **AfterCommit**: Called after successful commit (non-critical)
- **BeforeRollback**: Called before rollback (non-critical)
- **AfterRollback**: Called after rollback (non-critical)
- **BeforeOperation**: Called before each operation (critical by default)
- **AfterOperation**: Called after each operation (non-critical)

### Adding Hooks

```go
// Non-critical hook (errors logged but don't block)
tx.AddHook(atom.AfterCommit, func(ctx context.Context, t *atom.Transaction) error {
    fmt.Println("Transaction committed!")
    return nil
})

// Critical hook (errors block the operation)
tx.AddCriticalHook(atom.BeforeCommit, func(ctx context.Context, t *atom.Transaction) error {
    // Validation logic
    if someCondition {
        return errors.New("validation failed")
    }
    return nil
})

// Hook with default criticality based on type
tx.AddDefaultHook(atom.BeforeCommit, validationFunc)

// Operation-specific hooks
tx.AddOperationHook(atom.BeforeOperation, func(ctx context.Context, tx *atom.Transaction, opName string, opIndex int) error {
    log.Printf("Starting operation %s (index %d)", opName, opIndex)
    return nil
})
```

### Hook Management

```go
// Remove all hooks of a specific type
tx.ClearHooks(atom.BeforeCommit)

// Remove all hooks
tx.ClearAllHooks()
```

## Observability

Integrate with metrics and tracing systems using the Observer interface:

```go
// Define a custom observer
type MyObserver struct {
    metrics *prometheus.Registry
}

func (o *MyObserver) OnOperationStart(ctx context.Context, name string) {
    // Record operation start
}

func (o *MyObserver) OnOperationEnd(ctx context.Context, name string, duration time.Duration, err error) {
    // Record operation metrics
}

func (o *MyObserver) OnCommit(ctx context.Context) {
    // Record commit
}

func (o *MyObserver) OnRollback(ctx context.Context, errors []error) {
    // Record rollback
}

// Use the observer
tx := atom.NewTransaction()
tx.SetObserver(&MyObserver{})
```

Built-in observers:
- `NoOpObserver`: Does nothing (useful for testing)
- `MetricsObserver`: Collects basic metrics (operation count, success/failure, duration)

## Logging

By default, the transaction does not log anything. You can optionally provide a `*slog.Logger` to enable structured logging:

```go
import "log/slog"

tx := atom.NewTransaction()

// Enable logging with the default slog logger
tx.SetLogger(slog.Default())

// Or use a custom logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
tx.SetLogger(logger)
```

When a logger is configured, the transaction will log:
- Warnings for failed compensations during rollback
- Warnings for non-critical hook failures
- Info messages for compensation retries

## Error Handling

The package provides specific error types:

```go
import "go.alis.build/atom/errors"

// Check error types
var opErr *errors.OperationError
if errors.As(err, &opErr) {
    fmt.Printf("Operation '%s' failed\n", opErr.Operation)
}

var panicErr *errors.PanicError
if errors.As(err, &panicErr) {
    fmt.Printf("Panic occurred: %v\n", panicErr.Value)
}

// Check for critical hook failures
if errors.Is(err, errors.ErrHookFailed) {
    fmt.Println("A critical hook failed")
}

// RollbackError supports multi-error unwrapping (Go 1.20+)
var rollbackErr *errors.RollbackError
if errors.As(err, &rollbackErr) {
    for _, e := range rollbackErr.Unwrap() {
        fmt.Printf("Compensation error: %v\n", e)
    }
}
```

## Utility Methods

```go
// Check transaction state
tx.IsCommitted()   // true if committed
tx.IsRolledBack()  // true if rolled back
tx.IsPending()     // true if neither committed nor rolled back

// Get operation count
count := tx.OperationCount()

// Get operation history
history := tx.GetHistory()
for _, op := range history {
    fmt.Printf("Operation: %s, Duration: %v, Error: %v\n", op.Name, op.Duration, op.Error)
}
```

## Best Practices

1. **Always use defer for cleanup**
   ```go
   defer func() {
       if !tx.IsCommitted() {
           _ = tx.Rollback(ctx)
       }
   }()
   ```

2. **Make compensating functions idempotent** - They might be called multiple times (especially with retry enabled)

3. **Handle partial failures gracefully** - Rollback does best-effort cleanup

4. **Use operation names** - Helpful for debugging and logging

5. **Validate before commit** - Use BeforeCommit hooks for final validation

6. **Don't ignore rollback errors** - Log them for debugging

7. **Use timeouts for external calls** - Prevent operations from hanging indefinitely

8. **Configure retry for flaky compensations** - Network operations may need retries

## Thread Safety

The transaction is thread-safe for concurrent access to its methods. The implementation uses RWMutex and atomic operations to ensure safe concurrent access. However, the operations and compensating functions you provide should handle their own thread safety if needed.

## Limitations

- Compensating functions are best-effort - if they fail after all retries, the error is logged but rollback continues
- Not suitable for distributed transactions across multiple services
- Operations should be relatively quick - long-running operations should use context cancellation
- Savepoints cannot be used after commit or rollback
