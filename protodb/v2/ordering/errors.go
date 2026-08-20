package ordering

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrInvalidOrder is returned when an order-by expression cannot be parsed.
//
// Common causes include:
//   - Invalid field name syntax (e.g., starting with a number)
//   - Invalid direction keyword (must be "asc", "ASC", "desc", or "DESC")
//   - Malformed expression syntax
//
// Example usage:
//
//	order, err := ordering.NewOrder(input)
//	if err != nil {
//	    var invalidOrder ordering.ErrInvalidOrder
//	    if errors.As(err, &invalidOrder) {
//	        log.Printf("Invalid order: %v", err)
//	    }
//	}
//
// This error implements the gRPC status interface and returns codes.InvalidArgument.
type ErrInvalidOrder struct {
	order string // The original order-by expression that failed
	err   error  // The underlying parsing error
}

// Error returns a formatted error message including the order expression and underlying error.
func (e ErrInvalidOrder) Error() string {
	return fmt.Sprintf("invalid order(%s): %v", e.order, e.err)
}

// Is implements error matching for errors.Is().
// Returns true if target is an ErrInvalidOrder or matches the underlying error.
func (e ErrInvalidOrder) Is(target error) bool {
	var errInvalidOrder ErrInvalidOrder
	return errors.As(target, &errInvalidOrder) || errors.Is(e.err, target)
}

// GRPCStatus returns the gRPC status for this error.
// Returns codes.InvalidArgument to indicate a client error.
func (e ErrInvalidOrder) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}
