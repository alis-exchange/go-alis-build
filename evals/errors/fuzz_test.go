package errors

import (
	"errors"
	"testing"

	"go.alis.build/evals/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func FuzzToGRPCEvalError(f *testing.F) {
	f.Add("field-name")
	f.Add("")

	f.Fuzz(func(t *testing.T, field string) {
		err := suite.ErrUnknownEnvironment{Name: "env-" + field}
		got := ToGRPC(err)
		if got == nil {
			t.Fatal("ToGRPC returned nil")
		}
		st, ok := status.FromError(got)
		if !ok {
			t.Fatal("not a gRPC status")
		}
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v, want InvalidArgument", st.Code())
		}

		prefixed := ToGRPCf(field, err)
		if prefixed == nil {
			t.Fatal("ToGRPCf returned nil")
		}
		st2, ok := status.FromError(prefixed)
		if !ok || st2.Code() != codes.InvalidArgument {
			t.Fatalf("ToGRPCf code=%v", st2.Code())
		}
	})
}

func FuzzToGRPCPlainError(f *testing.F) {
	f.Add("plain message")

	f.Fuzz(func(t *testing.T, msg string) {
		got := ToGRPC(errors.New(msg))
		st, ok := status.FromError(got)
		if !ok {
			t.Fatal("not a gRPC status")
		}
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v, want InvalidArgument", st.Code())
		}
		if !IsEval(errors.New(msg)) {
			// expected: plain errors are not EvalError
		}
	})
}
