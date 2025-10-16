package atom_test

import (
	"context"
	"os"
	"testing"

	"go.alis.build/atom"
)

func TestNewTransaction(t *testing.T) {
	type action struct {
		name       string
		operation  atom.OperationFunc
		compensate atom.CompensatingFunc
		wantErr    bool
	}

	tests := []struct {
		ctx     context.Context
		name    string   // description of this test case
		actions []action // sequence of actions to perform
		want    *atom.Transaction
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txn := atom.NewTransaction(tt.ctx)
			for _, action := range tt.actions {
				err := txn.Do(tt.ctx, action.name, action.operation, action.compensate)
				if (err != nil) != action.wantErr {
					t.Errorf("Do() error = %v, wantErr %v", err, action.wantErr)
				}
			}

			// Rollback the transaction to clean up
			_ = txn.Rollback(tt.ctx)
		})
	}
}

func TestTransaction_Do(t *testing.T) {
	ctx := context.Background()

	txn := atom.NewTransaction(ctx)

	txn.Do(ctx, "WriteFile", func(ctx context.Context) error {
		// Write a dummy file
		if err := os.WriteFile("dummy.txt", []byte("Hello, World!"), 0o644); err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context) error {
		// Compensating function to delete the file
		if err := os.Remove("dummy.txt"); err != nil {
			return err
		}

		return nil
	})

	// Rollback the transaction
	if err := txn.Rollback(ctx); err != nil {
		t.Errorf("Rollback() error = %v", err)
	}

	// Verify the file was deleted
	if _, err := os.Stat("dummy.txt"); !os.IsNotExist(err) {
		t.Errorf("File was not deleted during rollback")
	}
}
