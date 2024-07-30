package validate

import (
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	// Operation must match operations/{operationId} format, where the operationId is a uuid
	OperationRegex = `^operations/[a-z0-9-]{36}$`
)

// Argument validates an argument and returns a grpc error if not valid.
func Argument(name string, value string, regex string) error {
	// validate the Name field using regex
	if !regexp.MustCompile(regex).MatchString(value) {
		return status.Errorf(
			codes.InvalidArgument,
			"%s (%s) is not of the right format: %s", name, value, regex)
	}
	return nil
}

// Required validate an argument to be not nil.
func Required(name string, message proto.Message) error {
	if message == nil {
		return status.Errorf(codes.InvalidArgument, "%s is required", name)
	}
	return nil
}
