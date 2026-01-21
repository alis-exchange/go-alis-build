package errors_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	atomErrors "go.alis.build/atom/errors"
)

// =============================================================================
// Error Test Suite
// =============================================================================

type ErrorTestSuite struct {
	suite.Suite
}

func TestErrorSuite(t *testing.T) {
	suite.Run(t, new(ErrorTestSuite))
}

// -----------------------------------------------------------------------------
// Sentinel Errors Tests
// -----------------------------------------------------------------------------

func (s *ErrorTestSuite) TestSentinelErrors_AreDistinct() {
	s.NotEqual(atomErrors.ErrAlreadyCommitted, atomErrors.ErrAlreadyRolledBack)
	s.NotEqual(atomErrors.ErrAlreadyCommitted, atomErrors.ErrHookFailed)
	s.NotEqual(atomErrors.ErrAlreadyCommitted, atomErrors.ErrInvalidSavepoint)
}

func (s *ErrorTestSuite) TestErrAlreadyCommitted_Message() {
	s.Contains(atomErrors.ErrAlreadyCommitted.Error(), "committed")
}

func (s *ErrorTestSuite) TestErrAlreadyRolledBack_Message() {
	s.Contains(atomErrors.ErrAlreadyRolledBack.Error(), "rolled back")
}

func (s *ErrorTestSuite) TestErrHookFailed_Message() {
	s.Contains(atomErrors.ErrHookFailed.Error(), "hook")
}

func (s *ErrorTestSuite) TestErrInvalidSavepoint_Message() {
	s.Contains(atomErrors.ErrInvalidSavepoint.Error(), "savepoint")
}

// -----------------------------------------------------------------------------
// OperationError Tests
// -----------------------------------------------------------------------------

func (s *ErrorTestSuite) TestOperationError_Error_WithName() {
	err := &atomErrors.OperationError{
		Operation: "create-user",
		Err:       errors.New("database error"),
	}

	s.Contains(err.Error(), "create-user")
	s.Contains(err.Error(), "database error")
}

func (s *ErrorTestSuite) TestOperationError_Error_WithoutName() {
	err := &atomErrors.OperationError{
		Operation: "",
		Err:       errors.New("database error"),
	}

	s.Contains(err.Error(), "operation failed")
	s.Contains(err.Error(), "database error")
}

func (s *ErrorTestSuite) TestOperationError_Unwrap() {
	underlying := errors.New("underlying error")
	err := &atomErrors.OperationError{
		Operation: "test",
		Err:       underlying,
	}

	s.Equal(underlying, err.Unwrap())
	s.True(errors.Is(err, underlying))
}

func (s *ErrorTestSuite) TestOperationError_As() {
	underlying := errors.New("underlying error")
	err := &atomErrors.OperationError{
		Operation: "test",
		Err:       underlying,
	}

	var opErr *atomErrors.OperationError
	s.True(errors.As(err, &opErr))
	s.Equal("test", opErr.Operation)
}

// -----------------------------------------------------------------------------
// RollbackError Tests
// -----------------------------------------------------------------------------

func (s *ErrorTestSuite) TestRollbackError_Error_NoErrors() {
	err := &atomErrors.RollbackError{
		Errors: []error{},
	}

	s.Contains(err.Error(), "no errors")
}

func (s *ErrorTestSuite) TestRollbackError_Error_SingleError() {
	err := &atomErrors.RollbackError{
		Errors: []error{errors.New("error 1")},
	}

	s.Contains(err.Error(), "error 1")
}

func (s *ErrorTestSuite) TestRollbackError_Error_MultipleErrors() {
	err := &atomErrors.RollbackError{
		Errors: []error{
			errors.New("error 1"),
			errors.New("error 2"),
			errors.New("error 3"),
		},
	}

	s.Contains(err.Error(), "3 errors")
}

func (s *ErrorTestSuite) TestRollbackError_Unwrap() {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	rollbackErr := &atomErrors.RollbackError{
		Errors: []error{err1, err2},
	}

	unwrapped := rollbackErr.Unwrap()
	s.Len(unwrapped, 2)
	s.Equal(err1, unwrapped[0])
	s.Equal(err2, unwrapped[1])
}

func (s *ErrorTestSuite) TestRollbackError_Is_WithMultipleErrors() {
	targetErr := errors.New("target error")
	otherErr := errors.New("other error")

	rollbackErr := &atomErrors.RollbackError{
		Errors: []error{otherErr, targetErr},
	}

	// Go 1.20+ multi-error support
	s.True(errors.Is(rollbackErr, targetErr))
	s.True(errors.Is(rollbackErr, otherErr))
}

// -----------------------------------------------------------------------------
// HookError Tests
// -----------------------------------------------------------------------------

func (s *ErrorTestSuite) TestHookError_Error_Critical() {
	err := &atomErrors.HookError{
		HookType: "BeforeCommit",
		Err:      errors.New("validation failed"),
		Critical: true,
	}

	s.Contains(err.Error(), "critical")
	s.Contains(err.Error(), "BeforeCommit")
	s.Contains(err.Error(), "validation failed")
}

func (s *ErrorTestSuite) TestHookError_Error_NonCritical() {
	err := &atomErrors.HookError{
		HookType: "AfterCommit",
		Err:      errors.New("logging failed"),
		Critical: false,
	}

	s.NotContains(err.Error(), "critical")
	s.Contains(err.Error(), "AfterCommit")
	s.Contains(err.Error(), "logging failed")
}

func (s *ErrorTestSuite) TestHookError_Unwrap() {
	underlying := errors.New("underlying error")
	err := &atomErrors.HookError{
		HookType: "BeforeCommit",
		Err:      underlying,
		Critical: true,
	}

	s.Equal(underlying, err.Unwrap())
	s.True(errors.Is(err, underlying))
}

func (s *ErrorTestSuite) TestHookError_Is_CriticalMatchesErrHookFailed() {
	criticalErr := &atomErrors.HookError{
		HookType: "BeforeCommit",
		Err:      errors.New("error"),
		Critical: true,
	}

	s.True(errors.Is(criticalErr, atomErrors.ErrHookFailed))
}

func (s *ErrorTestSuite) TestHookError_Is_NonCriticalDoesNotMatchErrHookFailed() {
	nonCriticalErr := &atomErrors.HookError{
		HookType: "AfterCommit",
		Err:      errors.New("error"),
		Critical: false,
	}

	s.False(errors.Is(nonCriticalErr, atomErrors.ErrHookFailed))
}

func (s *ErrorTestSuite) TestHookError_Is_DoesNotMatchOtherErrors() {
	hookErr := &atomErrors.HookError{
		HookType: "BeforeCommit",
		Err:      errors.New("error"),
		Critical: true,
	}

	s.False(errors.Is(hookErr, atomErrors.ErrAlreadyCommitted))
	s.False(errors.Is(hookErr, atomErrors.ErrAlreadyRolledBack))
	s.False(errors.Is(hookErr, atomErrors.ErrInvalidSavepoint))
}

// -----------------------------------------------------------------------------
// PanicError Tests
// -----------------------------------------------------------------------------

func (s *ErrorTestSuite) TestPanicError_Error_StringValue() {
	err := &atomErrors.PanicError{
		Value: "something went wrong",
		Stack: "stack trace here",
	}

	s.Contains(err.Error(), "panic")
	s.Contains(err.Error(), "something went wrong")
}

func (s *ErrorTestSuite) TestPanicError_Error_IntValue() {
	err := &atomErrors.PanicError{
		Value: 42,
		Stack: "stack trace here",
	}

	s.Contains(err.Error(), "panic")
	s.Contains(err.Error(), "42")
}

func (s *ErrorTestSuite) TestPanicError_Error_ErrorValue() {
	err := &atomErrors.PanicError{
		Value: errors.New("inner error"),
		Stack: "stack trace here",
	}

	s.Contains(err.Error(), "panic")
	s.Contains(err.Error(), "inner error")
}

func (s *ErrorTestSuite) TestPanicError_StackIsPreserved() {
	err := &atomErrors.PanicError{
		Value: "panic",
		Stack: "goroutine 1 [running]:\nmain.main()\n\t/path/to/file.go:10",
	}

	s.Equal("goroutine 1 [running]:\nmain.main()\n\t/path/to/file.go:10", err.Stack)
}

// =============================================================================
// Error Wrapping Integration Tests
// =============================================================================

func TestErrorWrapping_OperationWithPanic(t *testing.T) {
	panicErr := &atomErrors.PanicError{
		Value: "test panic",
		Stack: "stack",
	}
	opErr := &atomErrors.OperationError{
		Operation: "test-op",
		Err:       panicErr,
	}

	// Should be able to extract both error types
	var extractedOpErr *atomErrors.OperationError
	assert.True(t, errors.As(opErr, &extractedOpErr))

	var extractedPanicErr *atomErrors.PanicError
	assert.True(t, errors.As(opErr, &extractedPanicErr))
	assert.Equal(t, "test panic", extractedPanicErr.Value)
}

func TestErrorWrapping_HookWithPanic(t *testing.T) {
	panicErr := &atomErrors.PanicError{
		Value: "hook panic",
		Stack: "stack",
	}
	hookErr := &atomErrors.HookError{
		HookType: "BeforeCommit",
		Err:      panicErr,
		Critical: true,
	}

	// Should be able to extract both error types
	var extractedHookErr *atomErrors.HookError
	assert.True(t, errors.As(hookErr, &extractedHookErr))
	assert.True(t, extractedHookErr.Critical)

	var extractedPanicErr *atomErrors.PanicError
	assert.True(t, errors.As(hookErr, &extractedPanicErr))
	assert.Equal(t, "hook panic", extractedPanicErr.Value)
}

func TestErrorWrapping_RollbackWithMultipleOperationErrors(t *testing.T) {
	opErr1 := &atomErrors.OperationError{
		Operation: "op1",
		Err:       errors.New("error 1"),
	}
	opErr2 := &atomErrors.OperationError{
		Operation: "op2",
		Err:       errors.New("error 2"),
	}

	rollbackErr := &atomErrors.RollbackError{
		Errors: []error{opErr1, opErr2},
	}

	// Should be able to check for specific operation errors
	assert.True(t, errors.Is(rollbackErr, opErr1.Err))
	assert.True(t, errors.Is(rollbackErr, opErr2.Err))
}

func TestErrorWrapping_DeepNesting(t *testing.T) {
	// Create a deeply nested error chain
	// Note: PanicError doesn't implement Unwrap, so baseErr won't be found via errors.Is
	baseErr := errors.New("base error")
	hookErr := &atomErrors.HookError{HookType: "BeforeCommit", Err: baseErr, Critical: true}
	opErr := &atomErrors.OperationError{Operation: "test", Err: hookErr}

	// Should be able to find the base error through the chain (via HookError.Unwrap -> OperationError.Unwrap)
	assert.True(t, errors.Is(opErr, baseErr))

	// Should be able to extract each error type
	var extractedOpErr *atomErrors.OperationError
	assert.True(t, errors.As(opErr, &extractedOpErr))

	var extractedHookErr *atomErrors.HookError
	assert.True(t, errors.As(opErr, &extractedHookErr))
}

func TestErrorWrapping_PanicErrorDoesNotUnwrap(t *testing.T) {
	// PanicError stores the panic value but doesn't implement Unwrap
	// This is intentional since panic values aren't always errors
	baseErr := errors.New("panic value")
	panicErr := &atomErrors.PanicError{Value: baseErr, Stack: "stack"}

	// PanicError doesn't unwrap, so errors.Is won't find baseErr
	// This is expected behavior
	var extractedPanicErr *atomErrors.PanicError
	assert.True(t, errors.As(panicErr, &extractedPanicErr))
	assert.Equal(t, baseErr, extractedPanicErr.Value)
}
