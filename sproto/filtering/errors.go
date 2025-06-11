package filtering

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrInvalidFilter is returned when an invalid filter is provided.
type ErrInvalidFilter struct {
	filter string
	err    error
}

func (e ErrInvalidFilter) Error() string {
	return fmt.Sprintf("invalid filter(%s): %v", e.filter, e.err)
}
func (e ErrInvalidFilter) Is(target error) bool {
	var errInvalidFilter ErrInvalidFilter
	return errors.As(target, &errInvalidFilter) || errors.Is(e.err, target)
}
func (e ErrInvalidFilter) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidIdentifier is returned when an invalid identifier is provided.
type ErrInvalidIdentifier struct {
	identifier string
	err        error
}

func (e ErrInvalidIdentifier) Error() string {
	return fmt.Sprintf("invalid identifier(%s): %v", e.identifier, e.err)
}
func (e ErrInvalidIdentifier) Is(target error) bool {
	var errInvalidIdentifier ErrInvalidIdentifier
	return errors.As(target, &errInvalidIdentifier) || errors.Is(e.err, target)
}
func (e ErrInvalidIdentifier) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}
