package protodb

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

func TestIsAlreadyExists(t *testing.T) {
	if !IsAlreadyExists(status.Error(codes.AlreadyExists, "x")) {
		t.Fatal()
	}
	if IsAlreadyExists(nil) {
		t.Fatal()
	}
}
