package atom

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"go.alis.build/alog"
	"go.alis.build/atom/errors"
)

// CompensatingFunc is a function that undoes/compensates for an operation
type CompensatingFunc func(context.Context) error

// OperationFunc is the main operation to execute
type OperationFunc func(context.Context) error

// Transaction manages a sequence of operations with rollback capability
type Transaction struct {
	ctx        context.Context
	operations []operationRecord
	hooks      map[HookType][]Hook
	committed  bool
	rolledBack bool
	mu         sync.Mutex
}

// operationRecord tracks metadata about an executed operation
type operationRecord struct {
	name       string
	compensate CompensatingFunc
	executedAt time.Time
	duration   time.Duration
	err        error
}

// NewTransaction creates a new transaction with the given context
func NewTransaction(ctx context.Context) *Transaction {
	return &Transaction{
		ctx:        ctx,
		operations: make([]operationRecord, 0),
		hooks:      make(map[HookType][]Hook),
	}
}

// Do executes an operation and registers its compensating function
// If the operation fails, it automatically triggers a rollback
// The name parameter is optional (can be empty string)
func (tx *Transaction) Do(ctx context.Context, name string, operation OperationFunc, compensate CompensatingFunc) error {
	tx.mu.Lock()

	// Check if already finalized
	if tx.committed {
		tx.mu.Unlock()
		return errors.ErrAlreadyCommitted
	}
	if tx.rolledBack {
		tx.mu.Unlock()
		return errors.ErrAlreadyRolledBack
	}

	tx.mu.Unlock()

	// Execute the operation with panic recovery
	startTime := time.Now()
	err := tx.executeOperationSafe(ctx, operation)
	duration := time.Since(startTime)

	// Record the operation
	record := operationRecord{
		name:       name,
		compensate: compensate,
		executedAt: startTime,
		duration:   duration,
		err:        err,
	}

	tx.mu.Lock()
	tx.operations = append(tx.operations, record)
	tx.mu.Unlock()

	// If operation failed, return error
	if err != nil {
		return &errors.OperationError{
			Operation: name,
			Err:       err,
		}
	}

	return nil
}

// executeOperationSafe executes an operation with panic recovery
func (tx *Transaction) executeOperationSafe(ctx context.Context, operation OperationFunc) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &errors.PanicError{
				Value: r,
				Stack: string(debug.Stack()),
			}
		}
	}()

	return operation(ctx)
}

// Commit finalizes the transaction, making rollback no longer possible
// It executes BeforeCommit and AfterCommit hooks
func (tx *Transaction) Commit(ctx context.Context) error {
	tx.mu.Lock()

	if tx.committed {
		tx.mu.Unlock()
		return errors.ErrAlreadyCommitted
	}
	if tx.rolledBack {
		tx.mu.Unlock()
		return errors.ErrAlreadyRolledBack
	}

	tx.mu.Unlock()

	// Execute BeforeCommit hooks (critical by default)
	if err := tx.executeHooks(ctx, BeforeCommit); err != nil {
		return err
	}

	// Mark as committed
	tx.mu.Lock()
	tx.committed = true
	tx.mu.Unlock()

	// Execute AfterCommit hooks (non-critical, log failures)
	_ = tx.executeHooks(ctx, AfterCommit)

	return nil
}

// Rollback executes all compensating functions in reverse order (LIFO)
// It executes BeforeRollback and AfterRollback hooks
func (tx *Transaction) Rollback(ctx context.Context) error {
	tx.mu.Lock()

	if tx.committed {
		tx.mu.Unlock()
		return errors.ErrAlreadyCommitted
	}
	if tx.rolledBack {
		tx.mu.Unlock()
		return nil // Already rolled back, no-op
	}

	tx.mu.Unlock()

	// Execute BeforeRollback hooks (non-critical by default)
	_ = tx.executeHooks(ctx, BeforeRollback)

	// Execute compensating functions in reverse order
	var rollbackErrors []error

	tx.mu.Lock()
	ops := tx.operations
	tx.mu.Unlock()

	for i := len(ops) - 1; i >= 0; i-- {
		op := ops[i]
		if op.compensate == nil {
			continue
		}

		if err := tx.executeCompensateSafe(ctx, op.compensate); err != nil {
			// Log the error but continue with other rollbacks
			alog.Warnf(ctx, "rollback failed for operation '%s': %v\n", op.name, err)
			rollbackErrors = append(rollbackErrors, fmt.Errorf("operation '%s': %w", op.name, err))
		}
	}

	// Mark as rolled back
	tx.mu.Lock()
	tx.rolledBack = true
	tx.mu.Unlock()

	// Execute AfterRollback hooks (non-critical)
	_ = tx.executeHooks(ctx, AfterRollback)

	if len(rollbackErrors) > 0 {
		return &errors.RollbackError{Errors: rollbackErrors}
	}

	return nil
}

// executeCompensateSafe executes a compensating function with panic recovery
func (tx *Transaction) executeCompensateSafe(ctx context.Context, compensate CompensatingFunc) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &errors.PanicError{
				Value: r,
				Stack: string(debug.Stack()),
			}
		}
	}()

	return compensate(ctx)
}

// GetHistory returns a copy of the operation history for debugging
func (tx *Transaction) GetHistory() []OperationRecord {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	history := make([]OperationRecord, len(tx.operations))
	for i, op := range tx.operations {
		history[i] = OperationRecord{
			Name:       op.name,
			ExecutedAt: op.executedAt,
			Duration:   op.duration,
			Error:      op.err,
		}
	}
	return history
}

// OperationRecord is the public view of operation metadata
type OperationRecord struct {
	Name       string
	ExecutedAt time.Time
	Duration   time.Duration
	Error      error
}

// IsCommitted returns whether the transaction has been committed
func (tx *Transaction) IsCommitted() bool {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	return tx.committed
}

// IsRolledBack returns whether the transaction has been rolled back
func (tx *Transaction) IsRolledBack() bool {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	return tx.rolledBack
}
