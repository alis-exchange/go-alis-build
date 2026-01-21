package atom

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"go.alis.build/atom/errors"
)

// HookType represents the type of hook to execute
type HookType int

const (
	// BeforeCommit is called before committing the transaction
	// Critical by default - failures block commit
	BeforeCommit HookType = iota

	// AfterCommit is called after successful commit
	// Non-critical by default - already committed, can't undo
	AfterCommit

	// BeforeRollback is called before rolling back operations
	// Non-critical by default - we're already in failure mode
	BeforeRollback

	// AfterRollback is called after rollback completes
	// Non-critical by default - rollback already done
	AfterRollback

	// BeforeOperation is called before each operation executes
	// Critical by default - failures block the operation
	BeforeOperation

	// AfterOperation is called after each operation completes (success or failure)
	// Non-critical by default - operation already executed
	AfterOperation
)

// String returns the string representation of HookType
func (h HookType) String() string {
	switch h {
	case BeforeCommit:
		return "BeforeCommit"
	case AfterCommit:
		return "AfterCommit"
	case BeforeRollback:
		return "BeforeRollback"
	case AfterRollback:
		return "AfterRollback"
	case BeforeOperation:
		return "BeforeOperation"
	case AfterOperation:
		return "AfterOperation"
	default:
		return fmt.Sprintf("Unknown(%d)", h)
	}
}

// HookFunc is a function that gets called at specific points in the transaction lifecycle
type HookFunc func(context.Context, *Transaction) error

// Hook represents a registered hook with its criticality setting
type Hook struct {
	Fn       HookFunc
	Critical bool // If true, error blocks the operation
}

// isCriticalByDefault returns whether a hook type is critical by default
func (h HookType) isCriticalByDefault() bool {
	return h == BeforeCommit || h == BeforeOperation
}

// OperationHookFunc is a function that gets called before/after each operation
// It receives the operation name and index in addition to the standard hook parameters
type OperationHookFunc func(ctx context.Context, tx *Transaction, opName string, opIndex int) error

// OperationHook represents a registered operation hook with its criticality setting
type OperationHook struct {
	Fn       OperationHookFunc
	Critical bool
}

// executeOperationHooks runs all operation hooks of a given type
func (tx *Transaction) executeOperationHooks(ctx context.Context, hookType HookType, opName string, opIndex int) error {
	tx.mu.RLock()
	srcHooks := tx.operationHooks[hookType]
	if len(srcHooks) == 0 {
		tx.mu.RUnlock()
		return nil
	}
	hooks := make([]OperationHook, len(srcHooks))
	copy(hooks, srcHooks)
	tx.mu.RUnlock()

	var criticalError error

	for i, hook := range hooks {
		if err := tx.executeOperationHookSafe(ctx, hookType, i, hook, opName, opIndex); err != nil {
			if hook.Critical {
				criticalError = err
				break
			}
			tx.logWarn(ctx, "non-critical operation hook failed",
				slog.String("hook_type", hookType.String()),
				slog.Int("hook_index", i),
				slog.String("operation", opName),
				slog.Any("error", err))
		}
	}

	return criticalError
}

// executeOperationHookSafe executes a single operation hook with panic recovery
func (tx *Transaction) executeOperationHookSafe(ctx context.Context, hookType HookType, index int, hook OperationHook, opName string, opIndex int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &errors.HookError{
				HookType: fmt.Sprintf("%s[%d]", hookType, index),
				Err: &errors.PanicError{
					Value: r,
					Stack: string(debug.Stack()),
				},
				Critical: hook.Critical,
			}
		}
	}()

	if err := hook.Fn(ctx, tx, opName, opIndex); err != nil {
		return &errors.HookError{
			HookType: hookType.String(),
			Err:      err,
			Critical: hook.Critical,
		}
	}

	return nil
}

// executeHooks runs all hooks of a given type and handles errors appropriately
func (tx *Transaction) executeHooks(ctx context.Context, hookType HookType) error {
	// Copy hooks slice under lock to avoid race conditions
	tx.mu.RLock()
	srcHooks := tx.hooks[hookType]
	if len(srcHooks) == 0 {
		tx.mu.RUnlock()
		return nil
	}
	hooks := make([]Hook, len(srcHooks))
	copy(hooks, srcHooks)
	tx.mu.RUnlock()

	var criticalError error

	for i, hook := range hooks {
		if err := tx.executeHookSafe(ctx, hookType, i, hook); err != nil {
			// If it's a critical hook and it failed, store the error
			if hook.Critical {
				criticalError = err
				// Stop executing remaining hooks if a critical one fails
				break
			}
			// Non-critical hooks just log (if logger configured) and continue
			tx.logWarn(ctx, "non-critical hook failed",
				slog.String("hook_type", hookType.String()),
				slog.Int("hook_index", i),
				slog.Any("error", err))
		}
	}

	return criticalError
}

// executeHookSafe executes a single hook with panic recovery
func (tx *Transaction) executeHookSafe(ctx context.Context, hookType HookType, index int, hook Hook) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &errors.HookError{
				HookType: fmt.Sprintf("%s[%d]", hookType, index),
				Err: &errors.PanicError{
					Value: r,
					Stack: string(debug.Stack()),
				},
				Critical: hook.Critical,
			}
		}
	}()

	if err := hook.Fn(ctx, tx); err != nil {
		return &errors.HookError{
			HookType: hookType.String(),
			Err:      err,
			Critical: hook.Critical,
		}
	}

	return nil
}

// AddHook adds a non-critical hook to the transaction
func (tx *Transaction) AddHook(hookType HookType, fn HookFunc) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.hooks == nil {
		tx.hooks = make(map[HookType][]Hook)
	}

	tx.hooks[hookType] = append(tx.hooks[hookType], Hook{
		Fn:       fn,
		Critical: false,
	})
}

// AddCriticalHook adds a critical hook that will block the operation if it fails
func (tx *Transaction) AddCriticalHook(hookType HookType, fn HookFunc) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.hooks == nil {
		tx.hooks = make(map[HookType][]Hook)
	}

	tx.hooks[hookType] = append(tx.hooks[hookType], Hook{
		Fn:       fn,
		Critical: true,
	})
}

// AddDefaultHook adds a hook with default criticality based on hook type
func (tx *Transaction) AddDefaultHook(hookType HookType, fn HookFunc) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.hooks == nil {
		tx.hooks = make(map[HookType][]Hook)
	}

	tx.hooks[hookType] = append(tx.hooks[hookType], Hook{
		Fn:       fn,
		Critical: hookType.isCriticalByDefault(),
	})
}

// ClearHooks removes all hooks of a specific type
func (tx *Transaction) ClearHooks(hookType HookType) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.hooks != nil {
		delete(tx.hooks, hookType)
	}
}

// ClearAllHooks removes all registered hooks
func (tx *Transaction) ClearAllHooks() {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	tx.hooks = make(map[HookType][]Hook)
	tx.operationHooks = make(map[HookType][]OperationHook)
}

// AddOperationHook adds a non-critical operation hook
// Operation hooks are called before/after each Do() or DoWithOptions() call
func (tx *Transaction) AddOperationHook(hookType HookType, fn OperationHookFunc) {
	if hookType != BeforeOperation && hookType != AfterOperation {
		return // Only operation hook types are valid
	}

	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.operationHooks == nil {
		tx.operationHooks = make(map[HookType][]OperationHook)
	}

	tx.operationHooks[hookType] = append(tx.operationHooks[hookType], OperationHook{
		Fn:       fn,
		Critical: false,
	})
}

// AddCriticalOperationHook adds a critical operation hook that blocks the operation if it fails
func (tx *Transaction) AddCriticalOperationHook(hookType HookType, fn OperationHookFunc) {
	if hookType != BeforeOperation && hookType != AfterOperation {
		return // Only operation hook types are valid
	}

	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.operationHooks == nil {
		tx.operationHooks = make(map[HookType][]OperationHook)
	}

	tx.operationHooks[hookType] = append(tx.operationHooks[hookType], OperationHook{
		Fn:       fn,
		Critical: true,
	})
}
