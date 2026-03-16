package protodb

import (
	"errors"

	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SpannerErrorToStatus converts a Spanner error to a gRPC status error.
func SpannerErrorToStatus(err error) error {
	if err == nil {
		return nil
	}

	// If it's already a gRPC status, just forward it
	if st, ok := status.FromError(err); ok {
		return st.Err()
	}

	// If it's an APIError, extract its status
	if apiErr, ok := errors.AsType[*apierror.APIError](err); ok {
		if apiSt := apiErr.GRPCStatus(); apiSt != nil {
			return apiSt.Err()
		}
	}

	// Fallback to Internal
	return status.Errorf(codes.Internal, "%s", err)
}
