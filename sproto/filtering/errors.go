package filtering

import (
	"errors"
	"fmt"
)

type ErrInvalidFilter struct {
	filter string
	err    error
}

func (e ErrInvalidFilter) Error() string {
	return fmt.Sprintf("invalid filter(%s): %v", e.filter, e.err)
}

func (e ErrInvalidFilter) Is(target error) bool {
	var errInvalidFilter ErrInvalidFilter
	return errors.As(target, &errInvalidFilter)
}

type ErrInvalidIdentifier struct {
	identifier string
	err        error
}

func (e ErrInvalidIdentifier) Error() string {
	return fmt.Sprintf("invalid identifier(%s): %v", e.identifier, e.err)
}

func (e ErrInvalidIdentifier) Is(target error) bool {
	var errInvalidIdentifier ErrInvalidIdentifier
	return errors.As(target, &errInvalidIdentifier)
}
