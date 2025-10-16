# Atom

Atom implements the Transaction/Unit of Work pattern with compensating actions for ensuring atomicity in non-database operations like network requests, file I/O, and other side effects.

## Features

- **Automatic Rollback**: Failed operations trigger automatic rollback of all previous operations in reverse order (LIFO)
- **Panic Recovery**: Panics in operations or compensating functions are caught and handled gracefully
- **Hooks System**: Add callbacks at key points (BeforeCommit, AfterCommit, BeforeRollback, AfterRollback)
- **Critical Hooks**: Hooks can block operations (e.g., validation before commit)
- **Operation Metadata**: Track execution time, errors, and operation history
- **Thread-Safe**: Concurrent access is protected with mutexes
- **Context Support**: Full context propagation for cancellation and timeouts

## Installation

```bash
go get go.alis.build/atom
```

## Basic Usage

```go
ctx := context.Background()
tx := transaction.NewTransaction(ctx)

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

## Hooks

### Hook Types

- **BeforeCommit**: Called before commit (critical by default - failures block commit)
- **AfterCommit**: Called after successful commit (non-critical)
- **BeforeRollback**: Called before rollback (non-critical)
- **AfterRollback**: Called after rollback (non-critical)

### Adding Hooks

```go
// Non-critical hook (errors logged but don't block)
tx.AddHook(transaction.AfterCommit, func(ctx context.Context, t *transaction.Transaction) error {
    fmt.Println("Transaction committed!")
    return nil
})

// Critical hook (errors block the operation)
tx.AddCriticalHook(transaction.BeforeCommit, func(ctx context.Context, t *transaction.Transaction) error {
    // Validation logic
    if someCondition {
        return errors.New("validation failed")
    }
    return nil
})

// Hook with default criticality based on type
tx.AddDefaultHook(transaction.BeforeCommit, validationFunc)
```

## Error Handling

The package provides specific error types:

```go
// Check error types
var opErr *transaction.OperationError
if errors.As(err, &opErr) {
    fmt.Printf("Operation '%s' failed\n", opErr.Operation)
}

var panicErr *transaction.PanicError
if errors.As(err, &panicErr) {
    fmt.Printf("Panic occurred: %v\n", panicErr.Value)
}
```

## Operation History

Inspect what happened in the transaction:

```go
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

2. **Make compensating functions idempotent** - They might be called multiple times

3. **Handle partial failures gracefully** - Rollback does best-effort cleanup

4. **Use operation names** - Helpful for debugging and logging

5. **Validate before commit** - Use BeforeCommit hooks for final validation

6. **Don't ignore rollback errors** - Log them for debugging

## Thread Safety

The transaction is thread-safe for concurrent access to its methods. However, the operations and compensating functions you provide should handle their own thread safety if needed.

## Limitations

- Compensating functions are best-effort - if they fail, the error is logged but rollback continues
- Not suitable for distributed transactions across multiple services
- Operations should be relatively quick - long-running operations should use context cancellation