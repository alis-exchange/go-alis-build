package protodb

import (
	"errors"

	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
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

// GormErrorToStatus converts GORM and underlying Spanner errors to a gRPC status error.
func GormErrorToStatus(err error) error {
	if err == nil {
		return nil
	}

	// 1. Handle GORM-specific Sentinel Errors
	// GORM generates these before hitting Spanner, or translates empty results into them.
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return status.Error(codes.NotFound, "record not found")

	case errors.Is(err, gorm.ErrInvalidTransaction),
		errors.Is(err, gorm.ErrMissingWhereClause),
		errors.Is(err, gorm.ErrInvalidData):
		return status.Errorf(codes.InvalidArgument, "invalid database request: %s", err.Error())
	}

	// 2. Extract standard gRPC status
	// The Spanner client heavily utilizes standard gRPC status errors for things
	// like constraint violations (AlreadyExists) or transaction aborts (Aborted).
	// Because GORM wraps errors, we use status.FromError which natively unwraps.
	if st, ok := status.FromError(err); ok {
		return st.Err()
	}

	// 3. Extract wrapped Google API errors
	// Sometimes Spanner errors are wrapped as Google API errors.
	var apiErr *apierror.APIError
	if errors.As(err, &apiErr) {
		if apiSt := apiErr.GRPCStatus(); apiSt != nil {
			return apiSt.Err()
		}
	}

	// 4. Fallback to Internal
	// Note: In a strict production environment, you may want to return a generic
	// string here instead of err.Error() to prevent leaking Spanner schema details.
	return status.Errorf(codes.Internal, "internal database error: %v", err)
}
