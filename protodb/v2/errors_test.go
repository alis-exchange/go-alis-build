package protodb

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestIsNotFound checks that IsNotFound is true only for a NotFound status
// error, and false for other codes and for nil.
func TestIsNotFound(t *testing.T) {
	if !IsNotFound(status.Error(codes.NotFound, "x")) {
		t.Fatal()
	}
	if IsNotFound(status.Error(codes.Internal, "x")) {
		t.Fatal()
	}
	if IsNotFound(nil) {
		t.Fatal()
	}
}

// TestIsAlreadyExists checks that IsAlreadyExists is true for an
// AlreadyExists status error and false for nil.
func TestIsAlreadyExists(t *testing.T) {
	if !IsAlreadyExists(status.Error(codes.AlreadyExists, "x")) {
		t.Fatal()
	}
	if IsAlreadyExists(nil) {
		t.Fatal()
	}
}
