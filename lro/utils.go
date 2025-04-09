package lro

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
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

/*
parseStructPbValue parses a *structpb.Value to the respective underlying type

It returns the parsed value as an interface{}
  - Value_NullValue is parsed to nil
  - Value_StringValue is parsed to a string
  - Value_NumberValue is parsed to a float64
  - Value_BoolValue is parsed to a boolean
  - Value_ListValue is parsed to a []interface{}, where each item is parsed recursively
  - Value_StructValue is parsed to a map[string]interface{}, where each item is parsed recursively
*/
func parseStructPbValue(value *structpb.Value) interface{} {
	var res interface{}

	switch value.GetKind().(type) {
	case *structpb.Value_NullValue:
		res = nil
	case *structpb.Value_StringValue:
		res = value.GetStringValue()
	case *structpb.Value_NumberValue:
		res = value.GetNumberValue()
	case *structpb.Value_BoolValue:
		res = value.GetBoolValue()
	case *structpb.Value_ListValue:
		res = []interface{}{}
		for _, v := range value.GetListValue().GetValues() {
			val := parseStructPbValue(v)
			res = append(res.([]interface{}), val)
		}
	case *structpb.Value_StructValue:
		res = map[string]interface{}{}
		for k, v := range value.GetStructValue().GetFields() {
			val := parseStructPbValue(v)
			res.(map[string]interface{})[k] = val
		}
	}

	return res
}
