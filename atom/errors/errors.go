package errors

import (
	"errors"
	"fmt"
)

var (
	// ErrAlreadyCommitted is returned when attempting to commit or rollback an already committed transaction
	ErrAlreadyCommitted = errors.New("transaction already committed")

	// ErrAlreadyRolledBack is returned when attempting to commit or rollback an already rolled back transaction
	ErrAlreadyRolledBack = errors.New("transaction already rolled back")

	// ErrHookFailed is returned when a critical hook fails
	ErrHookFailed = errors.New("critical hook failed")

	// ErrInvalidSavepoint is returned when a savepoint is invalid or belongs to a different transaction
	ErrInvalidSavepoint = errors.New("invalid savepoint")
)

// OperationError wraps an error that occurred during operation execution
type OperationError struct {
	Operation string
	Err       error
}

func (e *OperationError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("operation '%s' failed: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("operation failed: %v", e.Err)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

// RollbackError wraps errors that occurred during rollback
type RollbackError struct {
	Errors []error
}

func (e *RollbackError) Error() string {
	if len(e.Errors) == 0 {
		return "rollback completed with no errors"
	}
	if len(e.Errors) == 1 {
		return fmt.Sprintf("rollback error: %v", e.Errors[0])
	}
	return fmt.Sprintf("rollback completed with %d errors: %v", len(e.Errors), e.Errors)
}

// Unwrap returns the list of errors for Go 1.20+ multi-error support
// This enables errors.Is() and errors.As() to work with all wrapped errors
func (e *RollbackError) Unwrap() []error {
	return e.Errors
}

// HookError wraps an error that occurred during hook execution
type HookError struct {
	HookType string
	Err      error
	Critical bool
}

func (e *HookError) Error() string {
	if e.Critical {
		return fmt.Sprintf("critical hook '%s' failed: %v", e.HookType, e.Err)
	}
	return fmt.Sprintf("hook '%s' failed: %v", e.HookType, e.Err)
}

// Unwrap returns the underlying error
func (e *HookError) Unwrap() error {
	return e.Err
}

// Is implements error matching for HookError
// This allows errors.Is(err, ErrHookFailed) to work for critical hook errors
func (e *HookError) Is(target error) bool {
	if target == ErrHookFailed && e.Critical {
		return true
	}
	return false
}

// PanicError wraps a recovered panic
type PanicError struct {
	Value interface{}
	Stack string
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("panic recovered: %v", e.Value)
}
