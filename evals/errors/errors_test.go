package errors

import (
	"errors"
	"testing"

	"go.alis.build/evals"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToGRPC_evalError(t *testing.T) {
	t.Parallel()

	err := evals.ErrInvalidCaseName{Case: "files-v2.missing"}
	got := ToGRPC(err)
	st, ok := status.FromError(got)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", st.Code())
	}
}

func TestToGRPCf_preservesCode(t *testing.T) {
	t.Parallel()

	err := evals.ErrDuplicateCase{Case: "missing-case"}
	got := ToGRPCf("case_ids", err)
	st, ok := status.FromError(got)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", st.Code())
	}
	if got := st.Message(); got != `case_ids: evals: duplicate case name "missing-case"` {
		t.Fatalf("message = %q", got)
	}
}

func TestIsEval(t *testing.T) {
	t.Parallel()

	if !IsEval(evals.ErrEmptySuiteName{}) {
		t.Fatal("expected ErrEmptySuiteName to implement EvalError")
	}
	for _, err := range []error{
		evals.ErrInvalidCaseName{Case: "bad.name"},
		evals.ErrDuplicateCase{Case: "dup"},
		evals.ErrInvalidConcurrency{Value: 0},
		evals.ErrNilCaseFunc{Case: "nil"},
	} {
		if !IsEval(err) {
			t.Fatalf("expected %T to implement EvalError", err)
		}
	}
	if IsEval(errors.New("plain")) {
		t.Fatal("plain error should not implement EvalError")
	}
}

func TestIsEval_streamErrors(t *testing.T) {
	t.Parallel()

	for _, err := range []error{
		evals.ErrNilStream{},
		evals.ErrNilStreamMessage{},
	} {
		if !IsEval(err) {
			t.Fatalf("expected %T to implement EvalError", err)
		}
	}
}

func TestCode(t *testing.T) {
	t.Parallel()

	if Code(evals.ErrEmptySuiteName{}) != codes.InvalidArgument {
		t.Fatalf("code = %v", Code(evals.ErrEmptySuiteName{}))
	}
}
