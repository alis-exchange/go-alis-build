package evals

import (
	"errors"
	"testing"

	"google.golang.org/grpc/status"
)

func TestConfigurationErrorContracts(t *testing.T) {
	t.Parallel()

	errs := []interface {
		error
		GRPCStatus() *status.Status
	}{
		ErrEmptySuiteName{},
		ErrInvalidCaseName{},
		ErrInvalidCaseName{Case: "bad.name"},
		ErrDuplicateCase{Case: "same"},
		ErrInvalidConcurrency{Value: 0},
		ErrNilReporter{},
		ErrNilCaseFunc{Case: "case"},
		ErrNilOption{},
		ErrNilStream{},
		ErrNilStreamMessage{},
	}
	for _, err := range errs {
		if err.Error() == "" {
			t.Errorf("%T.Error() is empty", err)
		}
		if err.GRPCStatus() == nil {
			t.Errorf("%T.GRPCStatus() is nil", err)
		}
		if !errors.Is(err, err) {
			t.Errorf("errors.Is(%T, itself) = false", err)
		}
	}
}

func TestConfigErrors_nilAndUnwrap(t *testing.T) {
	t.Parallel()

	var nilErrors *ConfigErrors
	if nilErrors.Error() == "" {
		t.Fatal("nil ConfigErrors.Error() is empty")
	}
	if nilErrors.Unwrap() != nil {
		t.Fatal("nil ConfigErrors.Unwrap() must be nil")
	}

	aggregate := &ConfigErrors{}
	aggregate.add(nil)
	aggregate.add(ErrNilOption{})
	if len(aggregate.Unwrap()) != 1 {
		t.Fatalf("Unwrap() length = %d, want 1", len(aggregate.Unwrap()))
	}
	if !errors.Is(aggregate, &ConfigErrors{}) {
		t.Fatal("ConfigErrors must match ConfigErrors")
	}
}
