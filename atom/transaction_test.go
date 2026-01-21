package atom_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.alis.build/atom"
	atomErrors "go.alis.build/atom/errors"
)

func TestNewTransaction(t *testing.T) {
	txn := atom.NewTransaction()
	if txn == nil {
		t.Fatal("NewTransaction() returned nil")
	}

	if txn.IsCommitted() {
		t.Error("New transaction should not be committed")
	}
	if txn.IsRolledBack() {
		t.Error("New transaction should not be rolled back")
	}
}

func TestTransaction_Do_Success(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	executed := false
	err := txn.Do(ctx, "test-op", func(ctx context.Context) error {
		executed = true
		return nil
	}, nil)

	if err != nil {
		t.Errorf("Do() error = %v, want nil", err)
	}
	if !executed {
		t.Error("Operation was not executed")
	}
}

func TestTransaction_Do_Error(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	expectedErr := errors.New("operation failed")
	err := txn.Do(ctx, "failing-op", func(ctx context.Context) error {
		return expectedErr
	}, nil)

	if err == nil {
		t.Fatal("Do() should return error")
	}

	var opErr *atomErrors.OperationError
	if !errors.As(err, &opErr) {
		t.Errorf("Error should be OperationError, got %T", err)
	}
	if opErr.Operation != "failing-op" {
		t.Errorf("Operation name = %q, want %q", opErr.Operation, "failing-op")
	}
}

func TestTransaction_Do_WithFileRollback(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	testFile := "test_dummy.txt"
	t.Cleanup(func() {
		os.Remove(testFile)
	})

	err := txn.Do(ctx, "WriteFile", func(ctx context.Context) error {
		return os.WriteFile(testFile, []byte("Hello, World!"), 0o644)
	}, func(ctx context.Context) error {
		return os.Remove(testFile)
	})

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatal("File should exist after operation")
	}

	// Rollback the transaction
	if err := txn.Rollback(ctx); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	// Verify the file was deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should be deleted after rollback")
	}
}

func TestTransaction_Commit(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	err := txn.Do(ctx, "op1", func(ctx context.Context) error {
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	err = txn.Commit(ctx)
	if err != nil {
		t.Errorf("Commit() error = %v", err)
	}

	if !txn.IsCommitted() {
		t.Error("Transaction should be committed")
	}
}

func TestTransaction_CommitTwice(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	_ = txn.Commit(ctx)
	err := txn.Commit(ctx)

	if !errors.Is(err, atomErrors.ErrAlreadyCommitted) {
		t.Errorf("Second Commit() should return ErrAlreadyCommitted, got %v", err)
	}
}

func TestTransaction_RollbackAfterCommit(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	_ = txn.Commit(ctx)
	err := txn.Rollback(ctx)

	if !errors.Is(err, atomErrors.ErrAlreadyCommitted) {
		t.Errorf("Rollback() after commit should return ErrAlreadyCommitted, got %v", err)
	}
}

func TestTransaction_DoAfterCommit(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	_ = txn.Commit(ctx)
	err := txn.Do(ctx, "op", func(ctx context.Context) error {
		return nil
	}, nil)

	if !errors.Is(err, atomErrors.ErrAlreadyCommitted) {
		t.Errorf("Do() after commit should return ErrAlreadyCommitted, got %v", err)
	}
}

func TestTransaction_DoAfterRollback(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	_ = txn.Rollback(ctx)
	err := txn.Do(ctx, "op", func(ctx context.Context) error {
		return nil
	}, nil)

	if !errors.Is(err, atomErrors.ErrAlreadyRolledBack) {
		t.Errorf("Do() after rollback should return ErrAlreadyRolledBack, got %v", err)
	}
}

func TestTransaction_RollbackTwice(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	err1 := txn.Rollback(ctx)
	err2 := txn.Rollback(ctx)

	if err1 != nil {
		t.Errorf("First Rollback() error = %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second Rollback() should be no-op, got error = %v", err2)
	}
}

func TestTransaction_RollbackOrder(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	var order []int

	for i := 1; i <= 3; i++ {
		i := i // capture
		_ = txn.Do(ctx, "", func(ctx context.Context) error {
			return nil
		}, func(ctx context.Context) error {
			order = append(order, i)
			return nil
		})
	}

	_ = txn.Rollback(ctx)

	// Should be LIFO order
	expected := []int{3, 2, 1}
	if len(order) != len(expected) {
		t.Fatalf("Rollback order length = %d, want %d", len(order), len(expected))
	}
	for i, v := range order {
		if v != expected[i] {
			t.Errorf("Rollback order[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestTransaction_PanicRecovery(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	err := txn.Do(ctx, "panic-op", func(ctx context.Context) error {
		panic("test panic")
	}, nil)

	if err == nil {
		t.Fatal("Do() should return error on panic")
	}

	var opErr *atomErrors.OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("Error should be OperationError, got %T", err)
	}

	var panicErr *atomErrors.PanicError
	if !errors.As(opErr.Err, &panicErr) {
		t.Errorf("Underlying error should be PanicError, got %T", opErr.Err)
	}
}

func TestTransaction_GetHistory(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	_ = txn.Do(ctx, "op1", func(ctx context.Context) error { return nil }, nil)
	_ = txn.Do(ctx, "op2", func(ctx context.Context) error { return nil }, nil)

	history := txn.GetHistory()
	if len(history) != 2 {
		t.Fatalf("History length = %d, want 2", len(history))
	}
	if history[0].Name != "op1" {
		t.Errorf("History[0].Name = %q, want %q", history[0].Name, "op1")
	}
	if history[1].Name != "op2" {
		t.Errorf("History[1].Name = %q, want %q", history[1].Name, "op2")
	}
}

func TestTransaction_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	txn := atom.NewTransaction()
	err := txn.Do(ctx, "op", func(ctx context.Context) error {
		return nil
	}, nil)

	if err == nil {
		t.Fatal("Do() should return error on cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Error should wrap context.Canceled, got %v", err)
	}
}

func TestTransaction_ConcurrentDo(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	var wg sync.WaitGroup
	var counter int32
	numOps := 100

	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = txn.Do(ctx, "", func(ctx context.Context) error {
				atomic.AddInt32(&counter, 1)
				return nil
			}, nil)
		}(i)
	}

	wg.Wait()

	if atomic.LoadInt32(&counter) != int32(numOps) {
		t.Errorf("Counter = %d, want %d", counter, numOps)
	}

	history := txn.GetHistory()
	if len(history) != numOps {
		t.Errorf("History length = %d, want %d", len(history), numOps)
	}
}

func TestTransaction_Hooks(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	var hookOrder []string

	txn.AddHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		hookOrder = append(hookOrder, "BeforeCommit")
		return nil
	})
	txn.AddHook(atom.AfterCommit, func(ctx context.Context, tx *atom.Transaction) error {
		hookOrder = append(hookOrder, "AfterCommit")
		return nil
	})

	_ = txn.Commit(ctx)

	expected := []string{"BeforeCommit", "AfterCommit"}
	if len(hookOrder) != len(expected) {
		t.Fatalf("Hook order length = %d, want %d", len(hookOrder), len(expected))
	}
	for i, v := range hookOrder {
		if v != expected[i] {
			t.Errorf("Hook order[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestTransaction_CriticalHookBlocksCommit(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	hookErr := errors.New("critical hook failed")
	txn.AddCriticalHook(atom.BeforeCommit, func(ctx context.Context, tx *atom.Transaction) error {
		return hookErr
	})

	err := txn.Commit(ctx)
	if err == nil {
		t.Fatal("Commit() should fail when critical hook fails")
	}

	var hookError *atomErrors.HookError
	if !errors.As(err, &hookError) {
		t.Errorf("Error should be HookError, got %T", err)
	}

	// Transaction should not be committed
	if txn.IsCommitted() {
		t.Error("Transaction should not be committed when critical hook fails")
	}
}

func TestTransaction_RollbackHooks(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	var hookOrder []string

	txn.AddHook(atom.BeforeRollback, func(ctx context.Context, tx *atom.Transaction) error {
		hookOrder = append(hookOrder, "BeforeRollback")
		return nil
	})
	txn.AddHook(atom.AfterRollback, func(ctx context.Context, tx *atom.Transaction) error {
		hookOrder = append(hookOrder, "AfterRollback")
		return nil
	})

	_ = txn.Rollback(ctx)

	expected := []string{"BeforeRollback", "AfterRollback"}
	if len(hookOrder) != len(expected) {
		t.Fatalf("Hook order length = %d, want %d", len(hookOrder), len(expected))
	}
	for i, v := range hookOrder {
		if v != expected[i] {
			t.Errorf("Hook order[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestRollbackError_Unwrap(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	rollbackErr := &atomErrors.RollbackError{
		Errors: []error{err1, err2},
	}

	unwrapped := rollbackErr.Unwrap()
	if len(unwrapped) != 2 {
		t.Fatalf("Unwrap() length = %d, want 2", len(unwrapped))
	}
	if unwrapped[0] != err1 || unwrapped[1] != err2 {
		t.Error("Unwrap() returned wrong errors")
	}
}

func TestHookError_Is(t *testing.T) {
	criticalErr := &atomErrors.HookError{
		HookType: "BeforeCommit",
		Err:      errors.New("test"),
		Critical: true,
	}

	nonCriticalErr := &atomErrors.HookError{
		HookType: "AfterCommit",
		Err:      errors.New("test"),
		Critical: false,
	}

	if !errors.Is(criticalErr, atomErrors.ErrHookFailed) {
		t.Error("Critical HookError should match ErrHookFailed")
	}
	if errors.Is(nonCriticalErr, atomErrors.ErrHookFailed) {
		t.Error("Non-critical HookError should not match ErrHookFailed")
	}
}

func TestTransaction_CommitWaitsForOperations(t *testing.T) {
	ctx := context.Background()
	txn := atom.NewTransaction()

	var opStarted, opFinished atomic.Bool
	var commitStarted atomic.Bool

	// Start a slow operation
	go func() {
		_ = txn.Do(ctx, "slow-op", func(ctx context.Context) error {
			opStarted.Store(true)
			time.Sleep(100 * time.Millisecond)
			opFinished.Store(true)
			return nil
		}, nil)
	}()

	// Wait for operation to start
	for !opStarted.Load() {
		time.Sleep(time.Millisecond)
	}

	// Try to commit while operation is running
	go func() {
		commitStarted.Store(true)
		_ = txn.Commit(ctx)
	}()

	// Wait for commit to start
	for !commitStarted.Load() {
		time.Sleep(time.Millisecond)
	}

	// Give commit a chance to run
	time.Sleep(10 * time.Millisecond)

	// Operation should finish before commit completes
	// (commit waits for executing operations)
	time.Sleep(150 * time.Millisecond)

	if !opFinished.Load() {
		t.Error("Operation should have finished")
	}
	if !txn.IsCommitted() {
		t.Error("Transaction should be committed after operation finishes")
	}
}
