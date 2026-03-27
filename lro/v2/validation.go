package lro

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// validateOperationName validates an operation resource name in the format operations/{id}.
func validateOperationName(name string) error {
	if name == "" {
		return status.Error(codes.InvalidArgument, "name is required")
	}
	if !strings.HasPrefix(name, "operations/") || len(name) <= len("operations/") {
		return status.Errorf(codes.InvalidArgument, "invalid operation name %q", name)
	}
	return nil
}
