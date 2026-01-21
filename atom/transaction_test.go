package atom_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"go.alis.build/atom"
	atomErrors "go.alis.build/atom/errors"
)

// =============================================================================
// Transaction Test Suite
// =============================================================================

type TransactionTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *TransactionTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestTransactionSuite(t *testing.T) {
	suite.Run(t, new(TransactionTestSuite))
}

// -----------------------------------------------------------------------------
// NewTransaction Tests
// -----------------------------------------------------------------------------

func (s *TransactionTestSuite) TestNewTransaction_ReturnsNonNil() {
	tx := atom.NewTransaction()
	s.NotNil(tx)
}

func (s *TransactionTestSuite) TestNewTransaction_InitialState() {
	tx := atom.NewTransaction()

	s.False(tx.IsCommitted(), "new transaction should not be committed")
	s.False(tx.IsRolledBack(), "new transaction should not be rolled back")
	s.True(tx.IsPending(), "new transaction should be pending")
	s.Equal(0, tx.OperationCount(), "new transaction should have no operations")
}

func (s *TransactionTestSuite) TestNewTransaction_HistoryIsEmpty() {
	tx := atom.NewTransaction()
	history := tx.GetHistory()
	s.Empty(history)
}

// -----------------------------------------------------------------------------
// Do Tests
// -----------------------------------------------------------------------------

func (s *TransactionTestSuite) TestDo_ExecutesOperation() {
	tx := atom.NewTransaction()
	executed := false

	err := tx.Do(s.ctx, "test-op", func(ctx context.Context) error {
		executed = true
		return nil
	}, nil)

	s.NoError(err)
	s.True(executed)
}

func (s *TransactionTestSuite) TestDo_RecordsOperation() {
	tx := atom.NewTransaction()

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, nil)

	s.Equal(2, tx.OperationCount())
	history := tx.GetHistory()
	s.Len(history, 2)
	s.Equal("op1", history[0].Name)
	s.Equal("op2", history[1].Name)
}

func (s *TransactionTestSuite) TestDo_ReturnsOperationError() {
	tx := atom.NewTransaction()
	expectedErr := errors.New("operation failed")

	err := tx.Do(s.ctx, "failing-op", func(ctx context.Context) error {
		return expectedErr
	}, nil)

	s.Error(err)
	var opErr *atomErrors.OperationError
	s.True(errors.As(err, &opErr))
	s.Equal("failing-op", opErr.Operation)
	s.True(errors.Is(err, expectedErr))
}

func (s *TransactionTestSuite) TestDo_WithEmptyName() {
	tx := atom.NewTransaction()

	err := tx.Do(s.ctx, "", func(ctx context.Context) error { return nil }, nil)

	s.NoError(err)
	history := tx.GetHistory()
	s.Equal("", history[0].Name)
}

func (s *TransactionTestSuite) TestDo_WithNilCompensate() {
	tx := atom.NewTransaction()

	err := tx.Do(s.ctx, "op", func(ctx context.Context) error { return nil }, nil)
	s.NoError(err)

	// Rollback should succeed even with nil compensate
	err = tx.Rollback(s.ctx)
	s.NoError(err)
}

func (s *TransactionTestSuite) TestDo_AfterCommit_ReturnsError() {
	tx := atom.NewTransaction()
	_ = tx.Commit(s.ctx)

	err := tx.Do(s.ctx, "op", func(ctx context.Context) error { return nil }, nil)

	s.True(errors.Is(err, atomErrors.ErrAlreadyCommitted))
}

func (s *TransactionTestSuite) TestDo_AfterRollback_ReturnsError() {
	tx := atom.NewTransaction()
	_ = tx.Rollback(s.ctx)

	err := tx.Do(s.ctx, "op", func(ctx context.Context) error { return nil }, nil)

	s.True(errors.Is(err, atomErrors.ErrAlreadyRolledBack))
}

func (s *TransactionTestSuite) TestDo_WithCancelledContext() {
	tx := atom.NewTransaction()
	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	err := tx.Do(ctx, "op", func(ctx context.Context) error { return nil }, nil)

	s.Error(err)
	s.True(errors.Is(err, context.Canceled))
}

func (s *TransactionTestSuite) TestDo_RecordsDuration() {
	tx := atom.NewTransaction()
	sleepDuration := 50 * time.Millisecond

	_ = tx.Do(s.ctx, "slow-op", func(ctx context.Context) error {
		time.Sleep(sleepDuration)
		return nil
	}, nil)

	history := tx.GetHistory()
	s.GreaterOrEqual(history[0].Duration, sleepDuration)
}

func (s *TransactionTestSuite) TestDo_RecordsError() {
	tx := atom.NewTransaction()
	expectedErr := errors.New("test error")

	_ = tx.Do(s.ctx, "failing-op", func(ctx context.Context) error {
		return expectedErr
	}, nil)

	history := tx.GetHistory()
	s.NotNil(history[0].Error)
	s.True(errors.Is(history[0].Error, expectedErr))
}

// -----------------------------------------------------------------------------
// DoWithOptions Tests
// -----------------------------------------------------------------------------

func (s *TransactionTestSuite) TestDoWithOptions_WithTimeout_Success() {
	tx := atom.NewTransaction()

	err := tx.DoWithOptions(s.ctx, "fast-op", atom.OperationOptions{
		Timeout: 1 * time.Second,
	}, func(ctx context.Context) error {
		return nil
	}, nil)

	s.NoError(err)
}

func (s *TransactionTestSuite) TestDoWithOptions_WithTimeout_Exceeded() {
	tx := atom.NewTransaction()

	err := tx.DoWithOptions(s.ctx, "slow-op", atom.OperationOptions{
		Timeout: 10 * time.Millisecond,
	}, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	}, nil)

	s.Error(err)
	s.True(errors.Is(err, context.DeadlineExceeded))
}

func (s *TransactionTestSuite) TestDoWithOptions_WithRetryOptions() {
	tx := atom.NewTransaction()

	err := tx.DoWithOptions(s.ctx, "op", atom.OperationOptions{
		CompensationRetry: atom.DefaultRetryOptions(),
	}, func(ctx context.Context) error {
		return nil
	}, func(ctx context.Context) error {
		return nil
	})

	s.NoError(err)
}

// -----------------------------------------------------------------------------
// Commit Tests
// -----------------------------------------------------------------------------

func (s *TransactionTestSuite) TestCommit_Success() {
	tx := atom.NewTransaction()
	_ = tx.Do(s.ctx, "op", func(ctx context.Context) error { return nil }, nil)

	err := tx.Commit(s.ctx)

	s.NoError(err)
	s.True(tx.IsCommitted())
	s.False(tx.IsPending())
}

func (s *TransactionTestSuite) TestCommit_Twice_ReturnsError() {
	tx := atom.NewTransaction()
	_ = tx.Commit(s.ctx)

	err := tx.Commit(s.ctx)

	s.True(errors.Is(err, atomErrors.ErrAlreadyCommitted))
}

func (s *TransactionTestSuite) TestCommit_AfterRollback_ReturnsError() {
	tx := atom.NewTransaction()
	_ = tx.Rollback(s.ctx)

	err := tx.Commit(s.ctx)

	s.True(errors.Is(err, atomErrors.ErrAlreadyRolledBack))
}

func (s *TransactionTestSuite) TestCommit_WaitsForRunningOperations() {
	tx := atom.NewTransaction()
	var opStarted, opFinished atomic.Bool

	go func() {
		_ = tx.Do(s.ctx, "slow-op", func(ctx context.Context) error {
			opStarted.Store(true)
			time.Sleep(50 * time.Millisecond)
			opFinished.Store(true)
			return nil
		}, nil)
	}()

	// Wait for operation to start
	for !opStarted.Load() {
		time.Sleep(time.Millisecond)
	}

	// Commit should wait for operation
	err := tx.Commit(s.ctx)

	s.NoError(err)
	s.True(opFinished.Load())
	s.True(tx.IsCommitted())
}

// -----------------------------------------------------------------------------
// Rollback Tests
// -----------------------------------------------------------------------------

func (s *TransactionTestSuite) TestRollback_ExecutesCompensationsInReverseOrder() {
	tx := atom.NewTransaction()
	var order []int

	for i := 1; i <= 5; i++ {
		i := i
		_ = tx.Do(s.ctx, fmt.Sprintf("op%d", i), func(ctx context.Context) error {
			return nil
		}, func(ctx context.Context) error {
			order = append(order, i)
			return nil
		})
	}

	err := tx.Rollback(s.ctx)

	s.NoError(err)
	s.Equal([]int{5, 4, 3, 2, 1}, order)
}

func (s *TransactionTestSuite) TestRollback_SkipsNilCompensations() {
	tx := atom.NewTransaction()
	var compensated []string

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensated = append(compensated, "op2")
		return nil
	})
	_ = tx.Do(s.ctx, "op3", func(ctx context.Context) error { return nil }, nil)

	err := tx.Rollback(s.ctx)

	s.NoError(err)
	s.Equal([]string{"op2"}, compensated)
}

func (s *TransactionTestSuite) TestRollback_ContinuesOnCompensationError() {
	tx := atom.NewTransaction()
	var compensated []string

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensated = append(compensated, "op1")
		return nil
	})
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		return errors.New("compensation failed")
	})
	_ = tx.Do(s.ctx, "op3", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensated = append(compensated, "op3")
		return nil
	})

	err := tx.Rollback(s.ctx)

	s.Error(err)
	var rollbackErr *atomErrors.RollbackError
	s.True(errors.As(err, &rollbackErr))
	s.Len(rollbackErr.Errors, 1)
	// Both op1 and op3 should have been compensated despite op2 failing
	s.Contains(compensated, "op1")
	s.Contains(compensated, "op3")
}

func (s *TransactionTestSuite) TestRollback_Twice_IsNoOp() {
	tx := atom.NewTransaction()
	compensateCount := 0

	_ = tx.Do(s.ctx, "op", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensateCount++
		return nil
	})

	err1 := tx.Rollback(s.ctx)
	err2 := tx.Rollback(s.ctx)

	s.NoError(err1)
	s.NoError(err2)
	s.Equal(1, compensateCount)
}

func (s *TransactionTestSuite) TestRollback_AfterCommit_ReturnsError() {
	tx := atom.NewTransaction()
	_ = tx.Commit(s.ctx)

	err := tx.Rollback(s.ctx)

	s.True(errors.Is(err, atomErrors.ErrAlreadyCommitted))
}

func (s *TransactionTestSuite) TestRollback_SetsState() {
	tx := atom.NewTransaction()

	_ = tx.Rollback(s.ctx)

	s.True(tx.IsRolledBack())
	s.False(tx.IsCommitted())
	s.False(tx.IsPending())
}

// -----------------------------------------------------------------------------
// Panic Recovery Tests
// -----------------------------------------------------------------------------

func (s *TransactionTestSuite) TestDo_RecoversPanic() {
	tx := atom.NewTransaction()

	err := tx.Do(s.ctx, "panic-op", func(ctx context.Context) error {
		panic("test panic")
	}, nil)

	s.Error(err)
	var opErr *atomErrors.OperationError
	s.True(errors.As(err, &opErr))
	var panicErr *atomErrors.PanicError
	s.True(errors.As(opErr.Err, &panicErr))
	s.Equal("test panic", panicErr.Value)
	s.NotEmpty(panicErr.Stack)
}

func (s *TransactionTestSuite) TestRollback_RecoversPanicInCompensation() {
	tx := atom.NewTransaction()

	_ = tx.Do(s.ctx, "op", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		panic("compensation panic")
	})

	err := tx.Rollback(s.ctx)

	s.Error(err)
	var rollbackErr *atomErrors.RollbackError
	s.True(errors.As(err, &rollbackErr))
}

// -----------------------------------------------------------------------------
// Concurrency Tests
// -----------------------------------------------------------------------------

func (s *TransactionTestSuite) TestConcurrent_Do() {
	tx := atom.NewTransaction()
	var wg sync.WaitGroup
	numOps := 100

	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = tx.Do(s.ctx, fmt.Sprintf("op%d", i), func(ctx context.Context) error {
				return nil
			}, nil)
		}(i)
	}

	wg.Wait()

	s.Equal(numOps, tx.OperationCount())
}

func (s *TransactionTestSuite) TestConcurrent_DoAndCommit() {
	tx := atom.NewTransaction()
	var wg sync.WaitGroup
	var opCount atomic.Int32

	// Start multiple operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := tx.Do(s.ctx, "op", func(ctx context.Context) error {
				time.Sleep(10 * time.Millisecond)
				return nil
			}, nil)
			if err == nil {
				opCount.Add(1)
			}
		}()
	}

	// Try to commit while operations are running
	time.Sleep(5 * time.Millisecond)
	_ = tx.Commit(s.ctx)

	wg.Wait()

	// Some operations should have completed
	s.Greater(opCount.Load(), int32(0))
}

// -----------------------------------------------------------------------------
// Logger Tests
// -----------------------------------------------------------------------------

func (s *TransactionTestSuite) TestSetLogger() {
	tx := atom.NewTransaction()
	logger := slog.Default()

	tx.SetLogger(logger)

	s.Equal(logger, tx.GetLogger())
}

func (s *TransactionTestSuite) TestGetLogger_DefaultIsNil() {
	tx := atom.NewTransaction()

	s.Nil(tx.GetLogger())
}

// =============================================================================
// Savepoint Test Suite
// =============================================================================

type SavepointTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *SavepointTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestSavepointSuite(t *testing.T) {
	suite.Run(t, new(SavepointTestSuite))
}

func (s *SavepointTestSuite) TestCreateSavepoint() {
	tx := atom.NewTransaction()
	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, nil)

	sp := tx.CreateSavepoint("checkpoint")

	s.NotNil(sp)
	s.Equal("checkpoint", sp.Name())
	s.Equal(2, sp.Index())
}

func (s *SavepointTestSuite) TestRollbackToSavepoint_PartialRollback() {
	tx := atom.NewTransaction()
	var compensated []string

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensated = append(compensated, "op1")
		return nil
	})
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensated = append(compensated, "op2")
		return nil
	})

	sp := tx.CreateSavepoint("checkpoint")

	_ = tx.Do(s.ctx, "op3", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensated = append(compensated, "op3")
		return nil
	})
	_ = tx.Do(s.ctx, "op4", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensated = append(compensated, "op4")
		return nil
	})

	err := tx.RollbackToSavepoint(s.ctx, sp)

	s.NoError(err)
	// Only op3 and op4 should be compensated (in reverse order)
	s.Equal([]string{"op4", "op3"}, compensated)
	// Transaction should still be pending
	s.True(tx.IsPending())
	s.Equal(2, tx.OperationCount())
}

func (s *SavepointTestSuite) TestRollbackToSavepoint_ThenContinue() {
	tx := atom.NewTransaction()
	var compensated []string

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensated = append(compensated, "op1")
		return nil
	})

	sp := tx.CreateSavepoint("checkpoint")

	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensated = append(compensated, "op2")
		return nil
	})

	_ = tx.RollbackToSavepoint(s.ctx, sp)

	// Add more operations after partial rollback
	_ = tx.Do(s.ctx, "op3", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		compensated = append(compensated, "op3")
		return nil
	})

	err := tx.Commit(s.ctx)

	s.NoError(err)
	s.Equal([]string{"op2"}, compensated) // Only op2 was rolled back
}

func (s *SavepointTestSuite) TestRollbackToSavepoint_NilSavepoint() {
	tx := atom.NewTransaction()

	err := tx.RollbackToSavepoint(s.ctx, nil)

	s.True(errors.Is(err, atomErrors.ErrInvalidSavepoint))
}

func (s *SavepointTestSuite) TestRollbackToSavepoint_WrongTransaction() {
	tx1 := atom.NewTransaction()
	tx2 := atom.NewTransaction()

	sp := tx1.CreateSavepoint("checkpoint")

	err := tx2.RollbackToSavepoint(s.ctx, sp)

	s.True(errors.Is(err, atomErrors.ErrInvalidSavepoint))
}

func (s *SavepointTestSuite) TestRollbackToSavepoint_AfterCommit() {
	tx := atom.NewTransaction()
	sp := tx.CreateSavepoint("checkpoint")
	_ = tx.Commit(s.ctx)

	err := tx.RollbackToSavepoint(s.ctx, sp)

	s.True(errors.Is(err, atomErrors.ErrAlreadyCommitted))
}

func (s *SavepointTestSuite) TestRollbackToSavepoint_AfterRollback() {
	tx := atom.NewTransaction()
	sp := tx.CreateSavepoint("checkpoint")
	_ = tx.Rollback(s.ctx)

	err := tx.RollbackToSavepoint(s.ctx, sp)

	s.True(errors.Is(err, atomErrors.ErrAlreadyRolledBack))
}

// =============================================================================
// Hook Test Suite
// =============================================================================

type HookTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *HookTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestHookSuite(t *testing.T) {
	suite.Run(t, new(HookTestSuite))
}

func (s *HookTestSuite) TestAddHook_BeforeCommit() {
	tx := atom.NewTransaction()
	called := false

	tx.AddHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		called = true
		return nil
	})

	_ = tx.Commit(s.ctx)

	s.True(called)
}

func (s *HookTestSuite) TestAddHook_AfterCommit() {
	tx := atom.NewTransaction()
	called := false

	tx.AddHook(atom.AfterCommit, func(ctx context.Context, tx *atom.Transaction) error {
		called = true
		return nil
	})

	_ = tx.Commit(s.ctx)

	s.True(called)
}

func (s *HookTestSuite) TestAddHook_BeforeRollback() {
	tx := atom.NewTransaction()
	called := false

	tx.AddHook(atom.BeforeRollback, func(ctx context.Context, tx *atom.Transaction) error {
		called = true
		return nil
	})

	_ = tx.Rollback(s.ctx)

	s.True(called)
}

func (s *HookTestSuite) TestAddHook_AfterRollback() {
	tx := atom.NewTransaction()
	called := false

	tx.AddHook(atom.AfterRollback, func(ctx context.Context, tx *atom.Transaction) error {
		called = true
		return nil
	})

	_ = tx.Rollback(s.ctx)

	s.True(called)
}

func (s *HookTestSuite) TestHookOrder() {
	tx := atom.NewTransaction()
	var order []string

	tx.AddHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		order = append(order, "BeforeCommit")
		return nil
	})
	tx.AddHook(atom.AfterCommit, func(ctx context.Context, tx *atom.Transaction) error {
		order = append(order, "AfterCommit")
		return nil
	})

	_ = tx.Commit(s.ctx)

	s.Equal([]string{"BeforeCommit", "AfterCommit"}, order)
}

func (s *HookTestSuite) TestCriticalHook_BlocksCommit() {
	tx := atom.NewTransaction()

	tx.AddCriticalHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		return errors.New("validation failed")
	})

	err := tx.Commit(s.ctx)

	s.Error(err)
	s.False(tx.IsCommitted())
	s.True(errors.Is(err, atomErrors.ErrHookFailed))
}

func (s *HookTestSuite) TestNonCriticalHook_DoesNotBlockCommit() {
	tx := atom.NewTransaction()

	tx.AddHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		return errors.New("non-critical error")
	})

	err := tx.Commit(s.ctx)

	s.NoError(err)
	s.True(tx.IsCommitted())
}

func (s *HookTestSuite) TestAddDefaultHook_BeforeCommit_IsCritical() {
	tx := atom.NewTransaction()

	tx.AddDefaultHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		return errors.New("error")
	})

	err := tx.Commit(s.ctx)

	s.Error(err)
	s.False(tx.IsCommitted())
}

func (s *HookTestSuite) TestAddDefaultHook_AfterCommit_IsNonCritical() {
	tx := atom.NewTransaction()

	tx.AddDefaultHook(atom.AfterCommit, func(ctx context.Context, tx *atom.Transaction) error {
		return errors.New("error")
	})

	err := tx.Commit(s.ctx)

	s.NoError(err)
	s.True(tx.IsCommitted())
}

func (s *HookTestSuite) TestClearHooks() {
	tx := atom.NewTransaction()
	called := false

	tx.AddHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		called = true
		return nil
	})

	tx.ClearHooks(atom.BeforeCommit)
	_ = tx.Commit(s.ctx)

	s.False(called)
}

func (s *HookTestSuite) TestClearAllHooks() {
	tx := atom.NewTransaction()
	beforeCalled := false
	afterCalled := false

	tx.AddHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		beforeCalled = true
		return nil
	})
	tx.AddHook(atom.AfterCommit, func(ctx context.Context, tx *atom.Transaction) error {
		afterCalled = true
		return nil
	})

	tx.ClearAllHooks()
	_ = tx.Commit(s.ctx)

	s.False(beforeCalled)
	s.False(afterCalled)
}

func (s *HookTestSuite) TestHook_RecoversPanic() {
	tx := atom.NewTransaction()

	tx.AddCriticalHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		panic("hook panic")
	})

	err := tx.Commit(s.ctx)

	s.Error(err)
	var hookErr *atomErrors.HookError
	s.True(errors.As(err, &hookErr))
	var panicErr *atomErrors.PanicError
	s.True(errors.As(hookErr.Err, &panicErr))
}

func (s *HookTestSuite) TestMultipleHooks_SameType() {
	tx := atom.NewTransaction()
	var order []int

	tx.AddHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		order = append(order, 1)
		return nil
	})
	tx.AddHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		order = append(order, 2)
		return nil
	})
	tx.AddHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		order = append(order, 3)
		return nil
	})

	_ = tx.Commit(s.ctx)

	s.Equal([]int{1, 2, 3}, order)
}

// =============================================================================
// Operation Hook Test Suite
// =============================================================================

type OperationHookTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *OperationHookTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestOperationHookSuite(t *testing.T) {
	suite.Run(t, new(OperationHookTestSuite))
}

func (s *OperationHookTestSuite) TestBeforeOperation_Called() {
	tx := atom.NewTransaction()
	var calledWith []string

	tx.AddOperationHook(atom.BeforeOperation, func(ctx context.Context, tx *atom.Transaction, opName string, opIndex int) error {
		calledWith = append(calledWith, opName)
		return nil
	})

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, nil)

	s.Equal([]string{"op1", "op2"}, calledWith)
}

func (s *OperationHookTestSuite) TestAfterOperation_Called() {
	tx := atom.NewTransaction()
	var calledWith []string

	tx.AddOperationHook(atom.AfterOperation, func(ctx context.Context, tx *atom.Transaction, opName string, opIndex int) error {
		calledWith = append(calledWith, opName)
		return nil
	})

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, nil)

	s.Equal([]string{"op1", "op2"}, calledWith)
}

func (s *OperationHookTestSuite) TestCriticalBeforeOperation_BlocksOperation() {
	tx := atom.NewTransaction()

	tx.AddCriticalOperationHook(atom.BeforeOperation, func(ctx context.Context, tx *atom.Transaction, opName string, opIndex int) error {
		return errors.New("blocked")
	})

	err := tx.Do(s.ctx, "op", func(ctx context.Context) error { return nil }, nil)

	s.Error(err)
	s.Equal(0, tx.OperationCount()) // Operation should not be recorded
}

func (s *OperationHookTestSuite) TestOperationHook_ReceivesIndex() {
	tx := atom.NewTransaction()
	var indices []int

	tx.AddOperationHook(atom.BeforeOperation, func(ctx context.Context, tx *atom.Transaction, opName string, opIndex int) error {
		indices = append(indices, opIndex)
		return nil
	})

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(s.ctx, "op3", func(ctx context.Context) error { return nil }, nil)

	s.Equal([]int{0, 1, 2}, indices)
}

// =============================================================================
// Observer Test Suite
// =============================================================================

type ObserverTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *ObserverTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestObserverSuite(t *testing.T) {
	suite.Run(t, new(ObserverTestSuite))
}

type mockObserver struct {
	operationStarts []string
	operationEnds   []string
	commitCalled    bool
	rollbackCalled  bool
	rollbackErrors  []error
}

func (m *mockObserver) OnOperationStart(ctx context.Context, name string) {
	m.operationStarts = append(m.operationStarts, name)
}

func (m *mockObserver) OnOperationEnd(ctx context.Context, name string, duration time.Duration, err error) {
	m.operationEnds = append(m.operationEnds, name)
}

func (m *mockObserver) OnCommit(ctx context.Context) {
	m.commitCalled = true
}

func (m *mockObserver) OnRollback(ctx context.Context, errors []error) {
	m.rollbackCalled = true
	m.rollbackErrors = errors
}

func (s *ObserverTestSuite) TestObserver_OnOperationStart() {
	tx := atom.NewTransaction()
	obs := &mockObserver{}
	tx.SetObserver(obs)

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, nil)

	s.Equal([]string{"op1", "op2"}, obs.operationStarts)
}

func (s *ObserverTestSuite) TestObserver_OnOperationEnd() {
	tx := atom.NewTransaction()
	obs := &mockObserver{}
	tx.SetObserver(obs)

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return nil }, nil)

	s.Equal([]string{"op1", "op2"}, obs.operationEnds)
}

func (s *ObserverTestSuite) TestObserver_OnCommit() {
	tx := atom.NewTransaction()
	obs := &mockObserver{}
	tx.SetObserver(obs)

	_ = tx.Commit(s.ctx)

	s.True(obs.commitCalled)
}

func (s *ObserverTestSuite) TestObserver_OnRollback() {
	tx := atom.NewTransaction()
	obs := &mockObserver{}
	tx.SetObserver(obs)

	_ = tx.Rollback(s.ctx)

	s.True(obs.rollbackCalled)
}

func (s *ObserverTestSuite) TestObserver_OnRollback_WithErrors() {
	tx := atom.NewTransaction()
	obs := &mockObserver{}
	tx.SetObserver(obs)

	_ = tx.Do(s.ctx, "op", func(ctx context.Context) error { return nil }, func(ctx context.Context) error {
		return errors.New("compensation error")
	})

	_ = tx.Rollback(s.ctx)

	s.True(obs.rollbackCalled)
	s.Len(obs.rollbackErrors, 1)
}

func (s *ObserverTestSuite) TestMetricsObserver() {
	tx := atom.NewTransaction()
	obs := atom.NewMetricsObserver()
	tx.SetObserver(obs)

	_ = tx.Do(s.ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(s.ctx, "op2", func(ctx context.Context) error { return errors.New("error") }, nil)
	_ = tx.Commit(s.ctx)

	s.Equal(int64(2), obs.OperationCount)
	s.Equal(int64(1), obs.SuccessCount)
	s.Equal(int64(1), obs.FailureCount)
	s.Equal(int64(1), obs.CommitCount)
}

// =============================================================================
// Retry Test Suite
// =============================================================================

type RetryTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *RetryTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestRetrySuite(t *testing.T) {
	suite.Run(t, new(RetryTestSuite))
}

func (s *RetryTestSuite) TestDefaultRetryOptions() {
	opts := atom.DefaultRetryOptions()

	s.Equal(3, opts.MaxRetries)
	s.Equal(100*time.Millisecond, opts.InitialDelay)
	s.Equal(2.0, opts.BackoffMultiplier)
	s.Equal(5*time.Second, opts.MaxDelay)
}

func (s *RetryTestSuite) TestRetry_SucceedsAfterFailures() {
	tx := atom.NewTransaction()
	attempts := 0

	_ = tx.DoWithOptions(s.ctx, "op", atom.OperationOptions{
		CompensationRetry: &atom.RetryOptions{
			MaxRetries:        3,
			InitialDelay:      1 * time.Millisecond,
			BackoffMultiplier: 1.0,
		},
	}, func(ctx context.Context) error {
		return nil
	}, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	err := tx.Rollback(s.ctx)

	s.NoError(err)
	s.Equal(3, attempts)
}

func (s *RetryTestSuite) TestRetry_FailsAfterMaxRetries() {
	tx := atom.NewTransaction()
	attempts := 0

	_ = tx.DoWithOptions(s.ctx, "op", atom.OperationOptions{
		CompensationRetry: &atom.RetryOptions{
			MaxRetries:        2,
			InitialDelay:      1 * time.Millisecond,
			BackoffMultiplier: 1.0,
		},
	}, func(ctx context.Context) error {
		return nil
	}, func(ctx context.Context) error {
		attempts++
		return errors.New("permanent error")
	})

	err := tx.Rollback(s.ctx)

	s.Error(err)
	s.Equal(3, attempts) // 1 initial + 2 retries
}

func (s *RetryTestSuite) TestRetry_RespectsContextCancellation() {
	ctx, cancel := context.WithCancel(s.ctx)
	tx := atom.NewTransaction()
	attempts := 0

	_ = tx.DoWithOptions(ctx, "op", atom.OperationOptions{
		CompensationRetry: &atom.RetryOptions{
			MaxRetries:        10,
			InitialDelay:      50 * time.Millisecond,
			BackoffMultiplier: 1.0,
		},
	}, func(ctx context.Context) error {
		return nil
	}, func(ctx context.Context) error {
		attempts++
		if attempts == 2 {
			cancel()
		}
		return errors.New("error")
	})

	err := tx.Rollback(ctx)

	s.Error(err)
	s.LessOrEqual(attempts, 3)
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestIntegration_FileOperations(t *testing.T) {
	ctx := context.Background()
	tx := atom.NewTransaction()

	testDir := t.TempDir()
	file1 := testDir + "/file1.txt"
	file2 := testDir + "/file2.txt"

	// Create files
	err := tx.Do(ctx, "create-file1", func(ctx context.Context) error {
		return os.WriteFile(file1, []byte("content1"), 0o644)
	}, func(ctx context.Context) error {
		return os.Remove(file1)
	})
	require.NoError(t, err)

	err = tx.Do(ctx, "create-file2", func(ctx context.Context) error {
		return os.WriteFile(file2, []byte("content2"), 0o644)
	}, func(ctx context.Context) error {
		return os.Remove(file2)
	})
	require.NoError(t, err)

	// Verify files exist
	_, err = os.Stat(file1)
	assert.NoError(t, err)
	_, err = os.Stat(file2)
	assert.NoError(t, err)

	// Rollback
	err = tx.Rollback(ctx)
	require.NoError(t, err)

	// Verify files are deleted
	_, err = os.Stat(file1)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(file2)
	assert.True(t, os.IsNotExist(err))
}

func TestIntegration_PartialFailure(t *testing.T) {
	ctx := context.Background()
	tx := atom.NewTransaction()

	testDir := t.TempDir()
	file1 := testDir + "/file1.txt"

	// First operation succeeds
	err := tx.Do(ctx, "create-file1", func(ctx context.Context) error {
		return os.WriteFile(file1, []byte("content1"), 0o644)
	}, func(ctx context.Context) error {
		return os.Remove(file1)
	})
	require.NoError(t, err)

	// Second operation fails
	err = tx.Do(ctx, "failing-op", func(ctx context.Context) error {
		return errors.New("operation failed")
	}, nil)
	require.Error(t, err)

	// Rollback should clean up file1
	err = tx.Rollback(ctx)
	require.NoError(t, err)

	// Verify file1 is deleted
	_, err = os.Stat(file1)
	assert.True(t, os.IsNotExist(err))
}

func TestIntegration_SavepointRecovery(t *testing.T) {
	ctx := context.Background()
	tx := atom.NewTransaction()

	testDir := t.TempDir()
	file1 := testDir + "/file1.txt"
	file2 := testDir + "/file2.txt"

	// Create file1
	err := tx.Do(ctx, "create-file1", func(ctx context.Context) error {
		return os.WriteFile(file1, []byte("content1"), 0o644)
	}, func(ctx context.Context) error {
		return os.Remove(file1)
	})
	require.NoError(t, err)

	// Create savepoint
	sp := tx.CreateSavepoint("after-file1")

	// Create file2
	err = tx.Do(ctx, "create-file2", func(ctx context.Context) error {
		return os.WriteFile(file2, []byte("content2"), 0o644)
	}, func(ctx context.Context) error {
		return os.Remove(file2)
	})
	require.NoError(t, err)

	// Rollback to savepoint (should only remove file2)
	err = tx.RollbackToSavepoint(ctx, sp)
	require.NoError(t, err)

	// file1 should still exist
	_, err = os.Stat(file1)
	assert.NoError(t, err)

	// file2 should be deleted
	_, err = os.Stat(file2)
	assert.True(t, os.IsNotExist(err))

	// Commit the remaining operations
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// file1 should still exist after commit
	_, err = os.Stat(file1)
	assert.NoError(t, err)
}

func TestIntegration_HooksWithValidation(t *testing.T) {
	ctx := context.Background()
	tx := atom.NewTransaction()

	var totalAmount int

	// Add validation hook
	tx.AddCriticalHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		if totalAmount < 0 {
			return errors.New("total amount cannot be negative")
		}
		return nil
	})

	// Add operations
	_ = tx.Do(ctx, "add-100", func(ctx context.Context) error {
		totalAmount += 100
		return nil
	}, func(ctx context.Context) error {
		totalAmount -= 100
		return nil
	})

	_ = tx.Do(ctx, "subtract-150", func(ctx context.Context) error {
		totalAmount -= 150
		return nil
	}, func(ctx context.Context) error {
		totalAmount += 150
		return nil
	})

	// Commit should fail due to validation
	err := tx.Commit(ctx)
	assert.Error(t, err)
	assert.False(t, tx.IsCommitted())

	// Rollback to restore state
	err = tx.Rollback(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, totalAmount)
}

func TestIntegration_ObserverMetrics(t *testing.T) {
	ctx := context.Background()
	tx := atom.NewTransaction()
	obs := atom.NewMetricsObserver()
	tx.SetObserver(obs)

	// Perform some operations
	_ = tx.Do(ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(ctx, "op2", func(ctx context.Context) error { return nil }, nil)
	_ = tx.Do(ctx, "op3", func(ctx context.Context) error { return errors.New("error") }, nil)

	// Rollback
	_ = tx.Rollback(ctx)

	// Verify metrics
	assert.Equal(t, int64(3), obs.OperationCount)
	assert.Equal(t, int64(2), obs.SuccessCount)
	assert.Equal(t, int64(1), obs.FailureCount)
	assert.Equal(t, int64(0), obs.CommitCount)
	assert.Equal(t, int64(1), obs.RollbackCount)
}

func TestIntegration_ConcurrentTransactions(t *testing.T) {
	var wg sync.WaitGroup
	numTransactions := 10
	results := make([]bool, numTransactions)

	for i := 0; i < numTransactions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			ctx := context.Background()
			tx := atom.NewTransaction()

			for j := 0; j < 5; j++ {
				_ = tx.Do(ctx, fmt.Sprintf("tx%d-op%d", idx, j), func(ctx context.Context) error {
					time.Sleep(time.Millisecond)
					return nil
				}, nil)
			}

			err := tx.Commit(ctx)
			results[idx] = err == nil
		}(i)
	}

	wg.Wait()

	for i, success := range results {
		assert.True(t, success, "Transaction %d should succeed", i)
	}
}
