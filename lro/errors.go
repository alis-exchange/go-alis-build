package lro

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when the requested operation does not exist.
type ErrNotFound struct {
	Operation string // unavailable locations
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.Operation)
}

// ErrWaitDeadlineExceeded is returned when the WaitOperation exceeds the specified, or default, timeout
type ErrWaitDeadlineExceeded struct {
	message string
}

func (e ErrWaitDeadlineExceeded) Error() string {
	return e.message
}

type InvalidOperationName struct {
	Name string // unavailable locations
}

func (e InvalidOperationName) Error() string {
	return fmt.Sprintf("%s is not a valid operation name (must start with 'operations/')", e.Name)
}

type ErrInvalidNextToken struct {
	nextToken string
}

func (e ErrInvalidNextToken) Error() string {
	return fmt.Sprintf("invalid nextToken (%s)", e.nextToken)
}

// EOF is the error returned when no more entities (such as children operations)
// are available. Functions should return EOF only to signal a graceful end read.
var EOF = errors.New("EOF")
