package protodb

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsNotFound returns true if err is a gRPC status error with NotFound code.
// Returns false for nil errors or any other code.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return status.Code(err) == codes.NotFound
}

// IsAlreadyExists returns true if err is a gRPC status error with AlreadyExists code.
// Returns false for nil errors or any other code.
func IsAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	return status.Code(err) == codes.AlreadyExists
}
