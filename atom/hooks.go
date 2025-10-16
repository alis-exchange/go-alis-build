package atom

import (
	"context"
	"fmt"
	"runtime/debug"

	"go.alis.build/alog"
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
	return h == BeforeCommit
}

// executeHooks runs all hooks of a given type and handles errors appropriately
func (tx *Transaction) executeHooks(ctx context.Context, hookType HookType) error {
	hooks := tx.hooks[hookType]
	if len(hooks) == 0 {
		return nil
	}

	var criticalError error

	for i, hook := range hooks {
		if err := tx.executeHookSafe(ctx, hookType, i, hook); err != nil {
			// If it's a critical hook and it failed, store the error
			if hook.Critical {
				criticalError = err
				// Stop executing remaining hooks if a critical one fails
				break
			}
			// Non-critical hooks just log and continue
			// In a real implementation, you'd use a proper logger
			alog.Warnf(ctx, "non-critical hook %s[%d] failed: %v\n", hookType, i, err)
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
