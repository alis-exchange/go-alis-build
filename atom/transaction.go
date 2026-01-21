package atom

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"go.alis.build/atom/errors"
)

// CompensatingFunc is a function that undoes/compensates for an operation
type CompensatingFunc func(context.Context) error

// OperationFunc is the main operation to execute
type OperationFunc func(context.Context) error

// OperationOptions configures behavior for an operation
type OperationOptions struct {
	// Timeout for the operation (0 means no timeout)
	Timeout time.Duration
	// CompensationRetry configures retry behavior for the compensating function
	CompensationRetry *RetryOptions
}

// RetryOptions configures retry behavior for compensating functions
type RetryOptions struct {
	// MaxRetries is the maximum number of retry attempts (0 means no retries)
	MaxRetries int
	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration
	// BackoffMultiplier is the multiplier for exponential backoff (e.g., 2.0 doubles delay each retry)
	BackoffMultiplier float64
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
}

// DefaultRetryOptions returns sensible default retry options
func DefaultRetryOptions() *RetryOptions {
	return &RetryOptions{
		MaxRetries:        3,
		InitialDelay:      100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		MaxDelay:          5 * time.Second,
	}
}

// Transaction manages a sequence of operations with rollback capability
type Transaction struct {
	operations     []operationRecord
	hooks          map[HookType][]Hook
	operationHooks map[HookType][]OperationHook
	committed      bool
	rolledBack     bool
	executing      int32 // atomic counter for operations in progress
	observer       Observer
	logger         *slog.Logger
	mu             sync.RWMutex
}

// operationRecord tracks metadata about an executed operation
type operationRecord struct {
	name       string
	compensate CompensatingFunc
	retryOpts  *RetryOptions
	executedAt time.Time
	duration   time.Duration
	err        error
}

// NewTransaction creates a new transaction
func NewTransaction() *Transaction {
	return &Transaction{
		operations: make([]operationRecord, 0),
		hooks:      make(map[HookType][]Hook),
	}
}

// Do executes an operation and registers its compensating function
// If the operation fails, it automatically triggers a rollback
// The name parameter is optional (can be empty string)
func (tx *Transaction) Do(ctx context.Context, name string, operation OperationFunc, compensate CompensatingFunc) error {
	// Check context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return &errors.OperationError{Operation: name, Err: err}
	}

	// Increment executing counter and check state atomically
	tx.mu.Lock()
	if tx.committed {
		tx.mu.Unlock()
		return errors.ErrAlreadyCommitted
	}
	if tx.rolledBack {
		tx.mu.Unlock()
		return errors.ErrAlreadyRolledBack
	}
	// Mark that we're executing an operation (prevents commit/rollback)
	atomic.AddInt32(&tx.executing, 1)
	opIndex := len(tx.operations)
	tx.mu.Unlock()

	// Ensure we decrement the counter when done
	defer atomic.AddInt32(&tx.executing, -1)

	// Execute BeforeOperation hooks
	if err := tx.executeOperationHooks(ctx, BeforeOperation, name, opIndex); err != nil {
		return err
	}

	// Notify observer
	if tx.observer != nil {
		tx.observer.OnOperationStart(ctx, name)
	}

	// Execute the operation with panic recovery
	startTime := time.Now()
	err := tx.executeOperationSafe(ctx, operation)
	duration := time.Since(startTime)

	// Notify observer
	if tx.observer != nil {
		tx.observer.OnOperationEnd(ctx, name, duration, err)
	}

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

	// Execute AfterOperation hooks (non-critical, always runs)
	_ = tx.executeOperationHooks(ctx, AfterOperation, name, opIndex)

	// If operation failed, return error
	if err != nil {
		return &errors.OperationError{
			Operation: name,
			Err:       err,
		}
	}

	return nil
}

// DoWithOptions executes an operation with additional options like timeout and retry configuration
func (tx *Transaction) DoWithOptions(ctx context.Context, name string, opts OperationOptions, operation OperationFunc, compensate CompensatingFunc) error {
	// Apply timeout if specified
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Check context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return &errors.OperationError{Operation: name, Err: err}
	}

	// Increment executing counter and check state atomically
	tx.mu.Lock()
	if tx.committed {
		tx.mu.Unlock()
		return errors.ErrAlreadyCommitted
	}
	if tx.rolledBack {
		tx.mu.Unlock()
		return errors.ErrAlreadyRolledBack
	}
	// Mark that we're executing an operation (prevents commit/rollback)
	atomic.AddInt32(&tx.executing, 1)
	opIndex := len(tx.operations)
	tx.mu.Unlock()

	// Ensure we decrement the counter when done
	defer atomic.AddInt32(&tx.executing, -1)

	// Execute BeforeOperation hooks
	if err := tx.executeOperationHooks(ctx, BeforeOperation, name, opIndex); err != nil {
		return err
	}

	// Notify observer
	if tx.observer != nil {
		tx.observer.OnOperationStart(ctx, name)
	}

	// Execute the operation with panic recovery
	startTime := time.Now()
	err := tx.executeOperationSafe(ctx, operation)
	duration := time.Since(startTime)

	// Notify observer
	if tx.observer != nil {
		tx.observer.OnOperationEnd(ctx, name, duration, err)
	}

	// Record the operation with retry options
	record := operationRecord{
		name:       name,
		compensate: compensate,
		retryOpts:  opts.CompensationRetry,
		executedAt: startTime,
		duration:   duration,
		err:        err,
	}

	tx.mu.Lock()
	tx.operations = append(tx.operations, record)
	tx.mu.Unlock()

	// Execute AfterOperation hooks (non-critical, always runs)
	_ = tx.executeOperationHooks(ctx, AfterOperation, name, opIndex)

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

	// Wait for any executing operations to complete
	for atomic.LoadInt32(&tx.executing) > 0 {
		tx.mu.Unlock()
		// Brief sleep to avoid busy-waiting
		time.Sleep(time.Millisecond)
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}
		tx.mu.Lock()
		// Re-check state after re-acquiring lock
		if tx.committed {
			tx.mu.Unlock()
			return errors.ErrAlreadyCommitted
		}
		if tx.rolledBack {
			tx.mu.Unlock()
			return errors.ErrAlreadyRolledBack
		}
	}

	tx.mu.Unlock()

	// Execute BeforeCommit hooks (critical by default)
	if err := tx.executeHooks(ctx, BeforeCommit); err != nil {
		return err
	}

	// Mark as committed
	tx.mu.Lock()
	tx.committed = true
	observer := tx.observer
	tx.mu.Unlock()

	// Notify observer
	if observer != nil {
		observer.OnCommit(ctx)
	}

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

	// Wait for any executing operations to complete
	for atomic.LoadInt32(&tx.executing) > 0 {
		tx.mu.Unlock()
		// Brief sleep to avoid busy-waiting
		time.Sleep(time.Millisecond)
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}
		tx.mu.Lock()
		// Re-check state after re-acquiring lock
		if tx.committed {
			tx.mu.Unlock()
			return errors.ErrAlreadyCommitted
		}
		if tx.rolledBack {
			tx.mu.Unlock()
			return nil
		}
	}

	// Copy operations slice while holding lock
	ops := make([]operationRecord, len(tx.operations))
	copy(ops, tx.operations)
	tx.mu.Unlock()

	// Execute BeforeRollback hooks (non-critical by default)
	_ = tx.executeHooks(ctx, BeforeRollback)

	// Execute compensating functions in reverse order
	var rollbackErrors []error

	for i := len(ops) - 1; i >= 0; i-- {
		op := ops[i]
		if op.compensate == nil {
			continue
		}

		if err := tx.executeCompensateWithRetry(ctx, op); err != nil {
			// Log the error if logger is configured, but continue with other rollbacks
			tx.logWarn(ctx, "rollback failed for operation",
				slog.String("operation", op.name),
				slog.Any("error", err))
			rollbackErrors = append(rollbackErrors, fmt.Errorf("operation '%s': %w", op.name, err))
		}
	}

	// Mark as rolled back
	tx.mu.Lock()
	tx.rolledBack = true
	observer := tx.observer
	tx.mu.Unlock()

	// Notify observer
	if observer != nil {
		observer.OnRollback(ctx, rollbackErrors)
	}

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

// executeCompensateWithRetry executes a compensating function with optional retry logic
func (tx *Transaction) executeCompensateWithRetry(ctx context.Context, op operationRecord) error {
	err := tx.executeCompensateSafe(ctx, op.compensate)
	if err == nil {
		return nil
	}

	// If no retry options, return the error immediately
	if op.retryOpts == nil || op.retryOpts.MaxRetries <= 0 {
		return err
	}

	// Retry with exponential backoff
	delay := op.retryOpts.InitialDelay
	for attempt := 1; attempt <= op.retryOpts.MaxRetries; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return fmt.Errorf("retry cancelled after %d attempts: %w", attempt-1, err)
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled after %d attempts: %w", attempt-1, err)
		case <-time.After(delay):
		}

		tx.logInfo(ctx, "retrying compensation for operation",
			slog.String("operation", op.name),
			slog.Int("attempt", attempt),
			slog.Int("max_retries", op.retryOpts.MaxRetries))

		err = tx.executeCompensateSafe(ctx, op.compensate)
		if err == nil {
			return nil
		}

		// Calculate next delay with exponential backoff
		if op.retryOpts.BackoffMultiplier > 0 {
			delay = time.Duration(float64(delay) * op.retryOpts.BackoffMultiplier)
			if op.retryOpts.MaxDelay > 0 && delay > op.retryOpts.MaxDelay {
				delay = op.retryOpts.MaxDelay
			}
		}
	}

	return fmt.Errorf("compensation failed after %d retries: %w", op.retryOpts.MaxRetries, err)
}

// GetHistory returns a copy of the operation history for debugging
func (tx *Transaction) GetHistory() []OperationRecord {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

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
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.committed
}

// IsRolledBack returns whether the transaction has been rolled back
func (tx *Transaction) IsRolledBack() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.rolledBack
}

// IsPending returns true if the transaction is neither committed nor rolled back
func (tx *Transaction) IsPending() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return !tx.committed && !tx.rolledBack
}

// OperationCount returns the number of recorded operations
func (tx *Transaction) OperationCount() int {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return len(tx.operations)
}

// Savepoint represents a point in the transaction that can be rolled back to
type Savepoint struct {
	name  string
	index int // operation index at the time of savepoint creation
	tx    *Transaction
}

// Name returns the savepoint name
func (sp *Savepoint) Name() string {
	return sp.name
}

// Index returns the operation index at the time of savepoint creation
func (sp *Savepoint) Index() int {
	return sp.index
}

// CreateSavepoint creates a savepoint at the current position in the transaction
// This allows partial rollback to this point using RollbackToSavepoint
func (tx *Transaction) CreateSavepoint(name string) *Savepoint {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

	return &Savepoint{
		name:  name,
		index: len(tx.operations),
		tx:    tx,
	}
}

// RollbackToSavepoint rolls back operations to the specified savepoint
// Operations after the savepoint are compensated in reverse order (LIFO)
// The transaction remains pending and can continue with new operations
func (tx *Transaction) RollbackToSavepoint(ctx context.Context, sp *Savepoint) error {
	if sp == nil {
		return errors.ErrInvalidSavepoint
	}
	if sp.tx != tx {
		return errors.ErrInvalidSavepoint
	}

	tx.mu.Lock()

	if tx.committed {
		tx.mu.Unlock()
		return errors.ErrAlreadyCommitted
	}
	if tx.rolledBack {
		tx.mu.Unlock()
		return errors.ErrAlreadyRolledBack
	}

	// Wait for any executing operations to complete
	for atomic.LoadInt32(&tx.executing) > 0 {
		tx.mu.Unlock()
		time.Sleep(time.Millisecond)
		if err := ctx.Err(); err != nil {
			return err
		}
		tx.mu.Lock()
		if tx.committed {
			tx.mu.Unlock()
			return errors.ErrAlreadyCommitted
		}
		if tx.rolledBack {
			tx.mu.Unlock()
			return errors.ErrAlreadyRolledBack
		}
	}

	// Validate savepoint is still valid
	if sp.index > len(tx.operations) {
		tx.mu.Unlock()
		return errors.ErrInvalidSavepoint
	}

	// Get operations to rollback (from current position back to savepoint)
	opsToRollback := make([]operationRecord, len(tx.operations)-sp.index)
	copy(opsToRollback, tx.operations[sp.index:])

	// Truncate operations to savepoint
	tx.operations = tx.operations[:sp.index]
	tx.mu.Unlock()

	// Execute compensating functions in reverse order
	var rollbackErrors []error

	for i := len(opsToRollback) - 1; i >= 0; i-- {
		op := opsToRollback[i]
		if op.compensate == nil {
			continue
		}

		if err := tx.executeCompensateWithRetry(ctx, op); err != nil {
			tx.logWarn(ctx, "rollback to savepoint failed for operation",
				slog.String("operation", op.name),
				slog.String("savepoint", sp.name),
				slog.Any("error", err))
			rollbackErrors = append(rollbackErrors, fmt.Errorf("operation '%s': %w", op.name, err))
		}
	}

	if len(rollbackErrors) > 0 {
		return &errors.RollbackError{Errors: rollbackErrors}
	}

	return nil
}

// SetLogger sets the logger for this transaction
// If set, the transaction will log warnings and info messages using this logger
// If nil (default), no logging is performed
func (tx *Transaction) SetLogger(logger *slog.Logger) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.logger = logger
}

// GetLogger returns the current logger, if any
func (tx *Transaction) GetLogger() *slog.Logger {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.logger
}

// logWarn logs a warning message if a logger is configured
func (tx *Transaction) logWarn(ctx context.Context, msg string, args ...any) {
	tx.mu.RLock()
	logger := tx.logger
	tx.mu.RUnlock()

	if logger != nil {
		logger.WarnContext(ctx, msg, args...)
	}
}

// logInfo logs an info message if a logger is configured
func (tx *Transaction) logInfo(ctx context.Context, msg string, args ...any) {
	tx.mu.RLock()
	logger := tx.logger
	tx.mu.RUnlock()

	if logger != nil {
		logger.InfoContext(ctx, msg, args...)
	}
}
