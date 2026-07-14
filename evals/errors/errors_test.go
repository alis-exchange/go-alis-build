package errors

import (
	"errors"
	"testing"

	"go.alis.build/evals"
	"go.alis.build/evals/registry"
	"go.alis.build/evals/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToGRPC_evalError(t *testing.T) {
	t.Parallel()

	err := registry.ErrUnknownCase{Name: "files-v2.missing"}
	got := ToGRPC(err)
	st, ok := status.FromError(got)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", st.Code())
	}
}

func TestToGRPCf_preservesCode(t *testing.T) {
	t.Parallel()

	err := suite.ErrUnknownEnvironment{Name: "missing-env"}
	got := ToGRPCf("case_ids", err)
	st, ok := status.FromError(got)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", st.Code())
	}
	if got := st.Message(); got != `case_ids: unknown environment "missing-env"` {
		t.Fatalf("message = %q", got)
	}
}

func TestIsEval(t *testing.T) {
	t.Parallel()

	if !IsEval(suite.ErrNilSuite{}) {
		t.Fatal("expected ErrNilSuite to implement EvalError")
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

	if Code(suite.ErrNilSuite{}) != codes.FailedPrecondition {
		t.Fatalf("code = %v", Code(suite.ErrNilSuite{}))
	}
}
