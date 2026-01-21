package filtering

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrInvalidFilter is returned when a filter expression cannot be parsed or converted to SQL.
//
// Common causes include:
//   - Syntax errors in the CEL expression
//   - Unsupported functions or operators
//   - Invalid function arguments
//   - Malformed expressions (unbalanced parentheses, missing operands)
//
// The error wraps the underlying parsing error for detailed diagnostics.
//
// Example usage:
//
//	stmt, err := parser.Parse(filter)
//	if err != nil {
//	    var invalidFilter filtering.ErrInvalidFilter
//	    if errors.As(err, &invalidFilter) {
//	        // Log or return the invalid filter error
//	        log.Printf("Invalid filter: %v", err)
//	    }
//	}
//
// This error implements the gRPC status interface and returns codes.InvalidArgument.
type ErrInvalidFilter struct {
	filter string // The original filter expression that failed
	err    error  // The underlying parsing error
}

// Error returns a formatted error message including the filter and underlying error.
func (e ErrInvalidFilter) Error() string {
	return fmt.Sprintf("invalid filter(%s): %v", e.filter, e.err)
}

// Is implements error matching for errors.Is().
// Returns true if target is an ErrInvalidFilter or matches the underlying error.
func (e ErrInvalidFilter) Is(target error) bool {
	var errInvalidFilter ErrInvalidFilter
	return errors.As(target, &errInvalidFilter) || errors.Is(e.err, target)
}

// GRPCStatus returns the gRPC status for this error.
// Returns codes.InvalidArgument to indicate a client error.
func (e ErrInvalidFilter) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidIdentifier is returned when an identifier cannot be registered with the parser.
//
// This error is typically returned by [Parser.DeclareIdentifier] when the identifier
// configuration is invalid or conflicts with existing declarations.
//
// Example usage:
//
//	err := parser.DeclareIdentifier(filtering.Timestamp("create_time"))
//	if err != nil {
//	    var invalidIdent filtering.ErrInvalidIdentifier
//	    if errors.As(err, &invalidIdent) {
//	        log.Printf("Invalid identifier: %v", err)
//	    }
//	}
//
// This error implements the gRPC status interface and returns codes.InvalidArgument.
type ErrInvalidIdentifier struct {
	identifier string // The identifier path that failed
	err        error  // The underlying error
}

// Error returns a formatted error message including the identifier and underlying error.
func (e ErrInvalidIdentifier) Error() string {
	return fmt.Sprintf("invalid identifier(%s): %v", e.identifier, e.err)
}

// Is implements error matching for errors.Is().
// Returns true if target is an ErrInvalidIdentifier or matches the underlying error.
func (e ErrInvalidIdentifier) Is(target error) bool {
	var errInvalidIdentifier ErrInvalidIdentifier
	return errors.As(target, &errInvalidIdentifier) || errors.Is(e.err, target)
}

// GRPCStatus returns the gRPC status for this error.
// Returns codes.InvalidArgument to indicate a client error.
func (e ErrInvalidIdentifier) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}
