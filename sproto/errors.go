package sproto

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// ErrNotFound is returned when the desired resource is not found in Spanner.
type ErrNotFound struct {
	RowKey string // unavailable locations
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.RowKey)
}
func (e ErrNotFound) Is(target error) bool {
	var errNotFound ErrNotFound
	return errors.As(target, &errNotFound)
}

// ErrInvalidFieldMask is returned when the field mask is invalid.
var ErrInvalidFieldMask = errors.New("invalid field mask")

// ErrMismatchedTypes is returned when the expected and actual types do not match.
type ErrMismatchedTypes struct {
	Expected reflect.Type
	Actual   reflect.Type
}

func (e ErrMismatchedTypes) Error() string {
	return fmt.Sprintf("expected %s, got %s", e.Expected, e.Actual)
}
func (e ErrMismatchedTypes) Is(target error) bool {
	var errMismatchedTypes ErrMismatchedTypes
	return errors.As(target, &errMismatchedTypes)
}

// ErrInvalidPageToken is returned when the page token is invalid.
type ErrInvalidPageToken struct {
	pageToken string
}

func (e ErrInvalidPageToken) Error() string {
	return fmt.Sprintf("invalid pageToken (%s)", e.pageToken)
}
func (e ErrInvalidPageToken) Is(target error) bool {
	var errInvalidPageToken ErrInvalidPageToken
	return errors.As(target, &errInvalidPageToken)
}

// ErrNegativePageSize is returned when the page size is less than 0.
type ErrNegativePageSize struct{}

func (e ErrNegativePageSize) Error() string {
	return "page size cannot be less than 0"
}
func (e ErrNegativePageSize) Is(target error) bool {
	var errNegativePageSize ErrNegativePageSize
	return errors.As(target, &errNegativePageSize)
}

// ErrInvalidArguments is returned when the arguments are invalid.
type ErrInvalidArguments struct {
	err    error
	fields []string
}

func (e ErrInvalidArguments) Error() string {
	return fmt.Sprintf("invalid arguments(%s): %v", strings.Join(e.fields, ", "), e.err)
}
func (e ErrInvalidArguments) Is(target error) bool {
	var errInvalidArguments ErrInvalidArguments
	return errors.As(target, &errInvalidArguments)
}
