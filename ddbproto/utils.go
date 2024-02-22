package ddbproto

import (
	"errors"
	"fmt"
	"reflect"

	"dario.cat/mergo"
	"github.com/mennanov/fmutils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// ErrNotFound is returned when the desired resource is not found in Bigtable.
type ErrNotFound struct {
	RowKey string // unavailable locations
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.RowKey)
}

var ErrInvalidFieldMask = errors.New("invalid field mask")

type ErrMismatchedTypes struct {
	Expected reflect.Type
	Actual   reflect.Type
}

func (e ErrMismatchedTypes) Error() string {
	return fmt.Sprintf("expected %s, got %s", e.Expected, e.Actual)
}

type ErrInvalidNextToken struct {
	nextToken string
}

func (e ErrInvalidNextToken) Error() string {
	return fmt.Sprintf("invalid nextToken (%s)", e.nextToken)
}

type ErrNegativePageSize struct{}

func (e ErrNegativePageSize) Error() string {
	return "page size cannot be less than 0"
}

// newEmptyMessage returns a new instance of the same type as the provided proto.Message
func newEmptyMessage(msg proto.Message) proto.Message {
	// Get the reflect.Type of the message
	msgType := reflect.TypeOf(msg)
	if msgType.Kind() == reflect.Ptr {
		msgType = msgType.Elem()
	}

	// Create a new instance of the message type using reflection
	newMsg := reflect.New(msgType).Interface().(proto.Message)
	return newMsg
}

// mergeUpdates merges the updates into the current message in line with the update mask
func mergeUpdates(current proto.Message, updates proto.Message, updateMask *fieldmaskpb.FieldMask) error {
	// If current and updates are different types, return an error
	if reflect.TypeOf(current) != reflect.TypeOf(updates) {
		return ErrMismatchedTypes{
			Expected: reflect.TypeOf(current),
			Actual:   reflect.TypeOf(updates),
		}
	}

	// If updates is nil, return nil
	if updates == nil {
		return nil
	}
	// If current is nil, return updates
	if current == nil {
		current = updates
		return nil
	}

	// If updates is empty, return nil
	if proto.Size(updates) == 0 {
		return nil
	}

	// Apply Update Mask if provided
	if updateMask != nil {
		updateMask.Normalize()
		if !updateMask.IsValid(current) {
			return ErrInvalidFieldMask
		}
		// Redact the request according to the provided field mask.
		fmutils.Prune(current, updateMask.GetPaths())
	}

	// Merge the updates into the current message
	err := mergo.Merge(current, updates)
	if err != nil {
		return err
	}

	return nil
}
