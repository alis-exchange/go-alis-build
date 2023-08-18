package lro

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/genproto/googleapis/longrunning"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
)

// origin is an interface that wraps the GetOperation method. This allows us
// to use the GetOperation method of the service from which the operation originated,
// which should implement this interface if it produces longrunning operations.
type origin interface {
	GetOperation(ctx context.Context, in *longrunning.GetOperationRequest, opts ...grpc.CallOption) (*longrunning.Operation, error)
}

// WaitOperation waits for the operation to complete. The metadataCallback parameter can be used to handle metadata
// provided by the operation. The origin is the service from which the operation originates and should implement the
// origin interface.
func WaitOperation(ctx context.Context, op *longrunningpb.Operation, origin origin, metadataCallback func(*anypb.Any) error) (*longrunningpb.Operation, error) {
	var err error
	for !op.GetDone() {
		time.Sleep(10 * time.Millisecond)
		op, err = origin.GetOperation(ctx, &longrunningpb.GetOperationRequest{Name: op.GetName()})
		if err != nil {
			rpcErr, ok := status.FromError(err)
			if !ok {
				if rpcErr.Code() == codes.Unauthenticated {
					return nil, fmt.Errorf("Your client connection may have reset before the long running operation completed.\nPlease follow the cloud build logs for completion status.")
				}
				return nil, fmt.Errorf(rpcErr.Message())
			}
			return nil, fmt.Errorf("%s", err.Error())
		}

		if op.GetError() != nil {
			return nil, fmt.Errorf(op.GetError().GetMessage())
		} else {
			// Marshal the anypb.Any metadata to RunDefineMetadata object
			if metadataCallback != nil && op.GetMetadata() != nil {
				err := metadataCallback(op.GetMetadata())
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return op, nil
}
