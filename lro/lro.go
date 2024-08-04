package lro

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/spanner"
	"github.com/google/uuid"
	"go.alis.build/lro/internal/validate"
	"go.alis.build/sproto"
	"golang.org/x/sync/errgroup"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	OperationColumnName = "Operation"
	// CheckpointHeaderKey is used to keep track of actual code related checkpoints in the context of Resumable
	// LROs.
	CheckpointHeaderKey = "x-alis-checkpoint"
	// OperationIdHeaderKey is use to indicate the the LRO already exists, and does not need to be created
	OperationIdHeaderKey = "x-alis-operation-id"
	// ChildOperationIdHeaderKey is used to store any child operation ids relevant to the main LRO.
	// These headers are used in the context of resumable LROs, and can be more than one LROs.  That is
	// a parent LRO can kick off a few child LROs which need to be completed before resuming.
	ChildOperationIdHeaderKey = "x-alis-child-operation-id"
)

type Client struct {
	spanner *sproto.Client
	table   string
}

/*
NewClient creates a new lro Client object. The function takes three arguments:

  - project: The Google Cloud project which hosts the Spanner database
  - instance: The name of the Spanner instance
  - database: The name of the Spanner database
  - table: The name of the Spanner table used to keep track of LROs.
*/
func NewClient(ctx context.Context, project string, instance string, database string, table string) (*Client, error) {
	// Establish sproto spanner connection with fine grained table-level role
	client, err := sproto.NewClient(ctx, project, instance, database, "")
	if err != nil {
		return nil, err
	}

	return &Client{
		spanner: client,
		table:   table,
	}, nil
}

/*
Close closes the underlying spanner.Client instance.
*/
func (c *Client) Close() {
	c.spanner.Close()
}

// CreateOptions provide additional, optional, parameters to the CreateOperation method.
type CreateOptions struct {
	Id       string        // Id is used to provide user defined operation Ids
	Metadata proto.Message // Metadata object as defined for the relevant LRO metadata response.
}

// CreateOperation stores a new long-running operation in spanner, with done=false
func (c *Client) CreateOperation(ctx context.Context, opts *CreateOptions) (*longrunningpb.Operation, error) {
	// create new unpopulated long-running operation
	op := &longrunningpb.Operation{}

	// set opts to empty struct if nil
	if opts == nil {
		opts = &CreateOptions{}
	}

	// Set the name if an id has been provided.
	if opts.Id != "" {
		op.Name = "operations/" + opts.Id
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	op.Name = "operations/" + id.String()

	// set metadata
	if opts.Metadata != nil {
		anyMeta, _ := anypb.New(opts.Metadata)
		op.Metadata = anyMeta
	}

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = c.spanner.InsertRow(ctx, c.table, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// GetOperation can be used directly in your GetOperation rpc method to return a long-running operation to a client
func (c *Client) GetOperation(ctx context.Context, operationName string) (*longrunningpb.Operation, error) {
	// validate arguments
	err := validate.Argument("name", operationName, validate.OperationRegex)
	if err != nil {
		return nil, err
	}

	// read operation resource from spanner
	op := &longrunningpb.Operation{}
	err = c.spanner.ReadProto(ctx, c.table, spanner.Key{operationName}, OperationColumnName, op, nil)
	if err != nil {
		if _, ok := err.(sproto.ErrNotFound); ok {
			// Handle the ErrNotFound case.
			return nil, ErrNotFound{
				Operation: operationName,
			}
		} else {
			// Handle other error types.
			return nil, fmt.Errorf("read operation from database: %w", err)
		}
	}

	return op, nil
}

// DeleteOperation deletes the operation row, indexed by operationName, from the spanner table.
func (c *Client) DeleteOperation(ctx context.Context, operationName string) (*emptypb.Empty, error) {
	// validate operation name
	err := validate.Argument("name", operationName, validate.OperationRegex)
	if err != nil {
		return nil, err
	}

	// validate existence of operation
	_, err = c.GetOperation(ctx, operationName)
	if err != nil {
		return nil, err
	}

	// delete operation
	err = c.spanner.DeleteRow(ctx, c.table, spanner.Key{operationName})
	if err != nil {
		return nil, fmt.Errorf("delete operation (%s): %w", operationName, err)
	}
	return &emptypb.Empty{}, nil
}

// WaitOperation can be used directly in your WaitOperation rpc method to wait for a long-running operation to complete.
// The metadataCallback parameter can be used to handle metadata provided by the operation.
// Note that if you do not specify a timeout, the timeout is set to 49 seconds.
func (c *Client) WaitOperation(ctx context.Context, req *longrunningpb.WaitOperationRequest, metadataCallback func(*anypb.Any)) (*longrunningpb.Operation, error) {
	timeout := req.GetTimeout()
	if timeout == nil {
		timeout = &durationpb.Duration{Seconds: 7 * 7}
	}
	startTime := time.Now()
	duration := time.Duration(timeout.Seconds*1e9 + int64(timeout.Nanos))

	// start loop to check if operation is done or timeout has passed
	var op *longrunningpb.Operation
	var err error
	for {
		op, err = c.GetOperation(ctx, req.GetName())
		if err != nil {
			return nil, err
		}
		if op.Done {
			return op, nil
		}
		if metadataCallback != nil && op.Metadata != nil {
			// If a metadata callback was provided, and metadata is available, pass the metadata to the callback.
			metadataCallback(op.GetMetadata())
		}

		timePassed := time.Since(startTime)
		if timeout != nil && timePassed > duration {
			return nil, ErrWaitDeadlineExceeded{timeout: timeout}
		}
		time.Sleep(1 * time.Second)
	}
}

// BatchWaitOperations is a batch version of the WaitOperation method.
func (c *Client) BatchWaitOperations(ctx context.Context, operations []string, timeout *durationpb.Duration) ([]*longrunningpb.Operation, error) {
	// iterate through the requests
	errs, ctx := errgroup.WithContext(ctx)
	results := make([]*longrunningpb.Operation, len(operations))
	for i, operation := range operations {
		i := i
		errs.Go(func() error {
			op, err := c.WaitOperation(ctx, &longrunningpb.WaitOperationRequest{
				Name:    operation,
				Timeout: timeout,
			}, nil)
			if err != nil {
				return err
			}
			results[i] = op

			return nil
		})
	}

	err := errs.Wait()
	if err != nil {
		return nil, err
	}

	return results, nil
}

// SetSuccessful updates an existing long-running operation's done field to true, sets the response and updates the
// metadata if provided.
func (c *Client) SetSuccessful(ctx context.Context, operationName string, response proto.Message, metadata proto.Message) (*longrunningpb.Operation, error) {
	// get operation
	op, err := c.GetOperation(ctx, operationName)
	if err != nil {
		return nil, err
	}

	// update done and result
	op.Done = true
	if response != nil {
		resultAny, err := anypb.New(response)
		if err != nil {
			return nil, err
		}
		op.Result = &longrunningpb.Operation_Response{Response: resultAny}
	}

	// update metadata if provided
	if metadata != nil {
		metaAny, err := anypb.New(metadata)
		if err != nil {
			return nil, err
		}
		op.Metadata = metaAny
	}

	// update in spanner by first deleting and then writing
	_, err = c.DeleteOperation(ctx, op.GetName())
	if err != nil {
		return nil, err
	}

	//  write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = c.spanner.InsertRow(ctx, c.table, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// SetFailed updates an existing long-running operation's done field to true, sets the error and updates the metadata
// if metaOptions.Update is true
func (c *Client) SetFailed(ctx context.Context, operationName string, error error, metadata proto.Message) (*longrunningpb.Operation, error) {
	// get operation
	op, err := c.GetOperation(ctx, operationName)
	if err != nil {
		return nil, err
	}

	// update operation fields
	op.Done = true
	if error == nil {
		error = fmt.Errorf("unknown error")
	}

	op.Result = &longrunningpb.Operation_Error{Error: &statuspb.Status{
		Code:    int32(codes.Unknown),
		Message: error.Error(),
		Details: nil,
	}}

	if metadata != nil {
		// convert metadata to Any type as per longrunning.Operation requirement.
		metaAny, err := anypb.New(metadata)
		if err != nil {
			return nil, err
		}
		op.Metadata = metaAny
	}

	// update in spanner by first deleting and then writing
	_, err = c.DeleteOperation(ctx, op.GetName())
	if err != nil {
		return nil, err
	}

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = c.spanner.InsertRow(ctx, c.table, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// UpdateMetadata updates an existing long-running operation's metadata.  Metadata typically
// contains progress information and common metadata such as create time.
func (c *Client) UpdateMetadata(ctx context.Context, operationName string, metadata proto.Message) (*longrunningpb.Operation, error) {
	// get operation
	op, err := c.GetOperation(ctx, operationName)
	if err != nil {
		return nil, err
	}

	// update metadata if required
	metaAny, err := anypb.New(metadata)
	if err != nil {
		return nil, err
	}
	op.Metadata = metaAny

	// update in spanner by first deleting and then writing
	_, err = c.DeleteOperation(ctx, op.GetName())
	if err != nil {
		return nil, err
	}

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = c.spanner.InsertRow(ctx, c.table, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// ForwardContext forwards the incoming context related to LROs and add them to the outgoing context.
// This method will forward the following headers if available:
//   - x-alis-checkpoint
//   - x-alis-operation-id
//   - x-alis-child-operation-id (which may be more than one, since a main LRO could invoke multiple child LROs)
func (c *Client) ForwardContext(ctx context.Context) context.Context {
	// In order to handle the resumable LRO design pattern, we add the relevant headers to the outgoing context.
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		keys := []string{CheckpointHeaderKey, OperationIdHeaderKey, ChildOperationIdHeaderKey}
		kvPairs := []string{}
		for _, k := range keys {
			for _, v := range md.Get(k) {
				kvPairs = append(kvPairs, []string{k, v}...)
			}
		}
		if len(kvPairs) > 0 {
			ctx = metadata.AppendToOutgoingContext(ctx, kvPairs...)
		}
	}
	return ctx
}
