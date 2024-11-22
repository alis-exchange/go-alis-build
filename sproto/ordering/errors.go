package ordering

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ErrInvalidOrder struct {
	order string
	err   error
}

func (e ErrInvalidOrder) Error() string {
	return fmt.Sprintf("invalid order(%s): %v", e.order, e.err)
}
func (e ErrInvalidOrder) Is(target error) bool {
	var errInvalidOrder ErrInvalidOrder
	return errors.As(target, &errInvalidOrder) || errors.Is(e.err, target)
}
func (e ErrInvalidOrder) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}
