package lro

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/grpc"
)

// origin is an interface that wraps the GetOperation method. This allows us
// to use the GetOperation method of the service from which the operation originated,
// which should implement this interface if it produces longrunning operations.
type LroService interface {
	GetOperation(ctx context.Context, in *longrunningpb.GetOperationRequest, opts ...grpc.CallOption) (*longrunningpb.Operation, error)
}

// Wait waits for the operation to complete.
//
// Arguments:
//   - operation: The full LRO resource name, of the format 'operations/*'
//   - service: The service from which the operation originates
//   - timeout: The period after which the method time outs and returns an error.
func WaitOperation(ctx context.Context, operation string, service LroService, timeout time.Duration,
) (*longrunningpb.Operation, error) {
	// Set the default timeout
	if timeout == 0 {
		timeout = time.Second * 77
	}
	startTime := time.Now()

	// start loop to check if operation is done or timeout has passed
	for {
		op, err := service.GetOperation(ctx, &longrunningpb.GetOperationRequest{Name: operation})
		if err != nil {
			return nil, err
		}
		if op.Done {
			return op, nil
		}

		timePassed := time.Since(startTime)
		if timePassed.Seconds() > timeout.Seconds() {
			return nil, ErrWaitDeadlineExceeded{
				message: fmt.Sprintf("operation (%s) exceeded timeout deadline of %0.0f seconds",
					operation, timeout.Seconds()),
			}
		}
		time.Sleep(777 * time.Millisecond)
	}
}
