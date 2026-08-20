package spanneradapter

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorToStatusPassthroughAndFallback(t *testing.T) {
	if ErrorToStatus(nil) != nil {
		t.Fatal("nil in, nil out")
	}
	if status.Code(ErrorToStatus(status.Error(codes.NotFound, "x"))) != codes.NotFound {
		t.Fatal()
	}
	if status.Code(ErrorToStatus(errors.New("weird"))) != codes.Internal {
		t.Fatal()
	}
}
