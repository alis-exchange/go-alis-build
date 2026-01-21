package sproto

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// sprotoError defines the interface for all sproto error types.
// All sproto errors implement the standard error interface, support error comparison
// via the Is method, and can be converted to a gRPC status for proper error propagation.
type sprotoError interface {
	Error() string
	Is(target error) bool
	GRPCStatus() *status.Status
}

// ErrNotFound is returned when the desired resource is not found in Spanner.
// It maps to gRPC status code [codes.NotFound].
type ErrNotFound struct {
	// RowKey is the identifier of the resource that was not found.
	RowKey string
	// err is the underlying error, if any.
	err error
}

// Error returns a human-readable description of the not found error.
// If RowKey is set, it includes the row key in the message.
func (e ErrNotFound) Error() string {
	if e.RowKey == "" {
		return fmt.Sprintf("not found: %v", e.err)
	}
	return fmt.Sprintf("%s not found", e.RowKey)
}

// Is reports whether target matches this error type or the underlying error.
func (e ErrNotFound) Is(target error) bool {
	var errNotFound ErrNotFound
	return errors.As(target, &errNotFound) || errors.Is(e.err, target)
}

// GRPCStatus returns the gRPC status representation of the error with [codes.NotFound].
func (e ErrNotFound) GRPCStatus() *status.Status {
	return status.New(codes.NotFound, e.Error())
}

// ErrInvalidFieldMask is returned when the provided field mask is invalid or malformed.
// This typically occurs when the field mask contains paths that do not exist in the target message.
var ErrInvalidFieldMask = errors.New("invalid field mask")

// ErrMismatchedTypes is returned when the expected and actual types do not match.
// This error is typically encountered during type assertions or when unmarshaling data
// into an incompatible type. It maps to gRPC status code [codes.InvalidArgument].
type ErrMismatchedTypes struct {
	// Expected is the type that was expected.
	Expected reflect.Type
	// Actual is the type that was actually provided.
	Actual reflect.Type
}

// Error returns a human-readable description of the type mismatch.
func (e ErrMismatchedTypes) Error() string {
	return fmt.Sprintf("expected %s, got %s", e.Expected, e.Actual)
}

// Is reports whether target matches this error type.
func (e ErrMismatchedTypes) Is(target error) bool {
	var errMismatchedTypes ErrMismatchedTypes
	return errors.As(target, &errMismatchedTypes)
}

// GRPCStatus returns the gRPC status representation of the error with [codes.InvalidArgument].
func (e ErrMismatchedTypes) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidPageToken is returned when the page token is invalid or cannot be decoded.
// This typically occurs when a client provides a malformed or expired page token
// in a paginated list request. It maps to gRPC status code [codes.InvalidArgument].
type ErrInvalidPageToken struct {
	// pageToken is the invalid token that was provided.
	pageToken string
}

// Error returns a human-readable description including the invalid page token.
func (e ErrInvalidPageToken) Error() string {
	return fmt.Sprintf("invalid pageToken (%s)", e.pageToken)
}

// Is reports whether target matches this error type.
func (e ErrInvalidPageToken) Is(target error) bool {
	var errInvalidPageToken ErrInvalidPageToken
	return errors.As(target, &errInvalidPageToken)
}

// GRPCStatus returns the gRPC status representation of the error with [codes.InvalidArgument].
func (e ErrInvalidPageToken) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNegativePageSize is returned when the page size is less than 0.
// Page size must be a non-negative integer in paginated list requests.
// It maps to gRPC status code [codes.InvalidArgument].
type ErrNegativePageSize struct{}

// Error returns a human-readable description of the invalid page size error.
func (e ErrNegativePageSize) Error() string {
	return "page size cannot be less than 0"
}

// Is reports whether target matches this error type.
func (e ErrNegativePageSize) Is(target error) bool {
	var errNegativePageSize ErrNegativePageSize
	return errors.As(target, &errNegativePageSize)
}

// GRPCStatus returns the gRPC status representation of the error with [codes.InvalidArgument].
func (e ErrNegativePageSize) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidArguments is returned when one or more request arguments are invalid.
// This is a general-purpose error for validation failures that don't fit other specific error types.
// It maps to gRPC status code [codes.InvalidArgument].
type ErrInvalidArguments struct {
	// err is the underlying error describing what is invalid.
	err error
	// fields contains the names of the invalid fields, if applicable.
	fields []string
}

// Error returns a human-readable description of the invalid arguments.
// If field names are specified, they are included in the message.
func (e ErrInvalidArguments) Error() string {
	if len(e.fields) == 0 {
		return fmt.Sprintf("invalid arguments: %v", e.err)
	}
	return fmt.Sprintf("invalid arguments(%s): %v", strings.Join(e.fields, ", "), e.err)
}

// Is reports whether target matches this error type or the underlying error.
func (e ErrInvalidArguments) Is(target error) bool {
	var errInvalidArguments ErrInvalidArguments
	return errors.As(target, &errInvalidArguments) || errors.Is(e.err, target)
}

// GRPCStatus returns the gRPC status representation of the error with [codes.InvalidArgument].
func (e ErrInvalidArguments) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrAlreadyExists is returned when attempting to create a resource that already exists in Spanner.
// This typically occurs when a Create operation is called with an identifier that is already in use.
// It maps to gRPC status code [codes.AlreadyExists].
type ErrAlreadyExists struct {
	// err is the underlying error with additional context.
	err error
}

// Error returns a human-readable description of the already exists error.
func (e ErrAlreadyExists) Error() string {
	return fmt.Sprintf("already exists: %v", e.err)
}

// Is reports whether target matches this error type or the underlying error.
func (e ErrAlreadyExists) Is(target error) bool {
	var errAlreadyExists ErrAlreadyExists
	return errors.As(target, &errAlreadyExists) || errors.Is(e.err, target)
}

// GRPCStatus returns the gRPC status representation of the error with [codes.AlreadyExists].
func (e ErrAlreadyExists) GRPCStatus() *status.Status {
	return status.New(codes.AlreadyExists, e.Error())
}
