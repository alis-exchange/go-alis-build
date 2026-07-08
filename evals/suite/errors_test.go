package suite

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrNilSuite_GRPCStatus(t *testing.T) {
	t.Parallel()

	st := ErrNilSuite{}.GRPCStatus()
	if st.Code() != codes.FailedPrecondition {
		t.Fatalf("code = %v", st.Code())
	}
}

func TestErrUnknownEnvironment_GRPCStatus(t *testing.T) {
	t.Parallel()

	err := ErrUnknownEnvironment{Name: "files-v2"}
	st, ok := status.FromError(err.GRPCStatus().Err())
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v", st.Code())
	}
}

func TestErrInvalidFilterPath_GRPCStatus(t *testing.T) {
	t.Parallel()

	err := ErrInvalidFilterPath{Path: "a.b.c"}
	st, ok := status.FromError(err.GRPCStatus().Err())
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v", st.Code())
	}
}
