package lro

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// origin is an interface that wraps the GetOperation method. This allows us
// to use the GetOperation method of the service from which the operation originated,
// which should implement this interface if it produces longrunning operations.
type OperationsService interface {
	GetOperation(ctx context.Context, in *longrunningpb.GetOperationRequest, opts ...grpc.CallOption) (*longrunningpb.Operation, error)
}

// Wait waits for the operation to complete.
//
// Arguments:
//   - operation: The full LRO resource name, of the format 'operations/*'
//   - service: The service from which the operation originates
//   - timeout: The period after which the method time outs and returns an error.
//
// Before using this method consider using the op.Wait() method with the lro.WithClient() option to use a custom LRO client.
func WaitOperation(ctx context.Context, operation string, service OperationsService, timeout time.Duration,
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

// UnmarshalOperation retrieves the underlying long-running operation (LRO) and unmarshals its response and metadata
// into the provided protocol buffer messages.
//
// Parameters:
//   - operation: The resource name of the operation in the format `operations/*`.
//   - response: The protocol buffer message into which the response of the LRO should be unmarshalled. Can be nil.
//   - metadata: The protocol buffer message into which the metadata of the LRO should be unmarshalled. Can be nil.
//
// Returns:
//   - An error if the operation is not done, the operation resulted in an error, or there was an issue unmarshalling
//     the response or metadata. Nil otherwise.
func UnmarshalOperation(operation *longrunningpb.Operation, response, metadata proto.Message) error {
	// Unmarshal the Response
	if response != nil && operation.GetResponse() != nil {

		err := operation.GetResponse().UnmarshalTo(response)
		if err != nil {
			return err
		}
	}

	// Unmarshal the Metadata
	if metadata != nil && operation.GetMetadata() != nil {
		err := operation.GetMetadata().UnmarshalTo(metadata)
		if err != nil {
			return err
		}
	}

	// Return an error if not done
	if !operation.Done {
		return fmt.Errorf("operation (%s) is not done", operation)
	}

	// Also return an error if the result is an error
	if operation.GetError() != nil {
		return fmt.Errorf("%d: %s", operation.GetError().GetCode(), operation.GetError().GetMessage())
	}

	return nil
}
