package lro

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/spanner"
	executions "cloud.google.com/go/workflows/executions/apiv1"
	"cloud.google.com/go/workflows/executions/apiv1/executionspb"
	"github.com/google/uuid"
	"go.alis.build/lro/internal/validate"
	"go.alis.build/sproto"
	"golang.org/x/sync/errgroup"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	// OperationColumnName is the column name used in spanner to store LROs
	OperationColumnName = "Operation"
	// CheckpointColumnName is the column name used in spanner to store checkpoints (if used)
	CheckpointColumnName = "Checkpoint"
	// OperationIdHeaderKey is use to indicate the the LRO already exists, and does not need to be created
	OperationIdHeaderKey = "x-alis-operation-id"
)

type Client struct {
	spanner         *sproto.Client
	workflows       *executions.Client
	spannerConfig   *SpannerConfig
	WorkflowsConfig *WorkflowsConfig
}

// WorkflowsConfig is used to configre the underlying Google Workflows client.
type WorkflowsConfig struct {
	// Name of the workflow for which an execution should be created.
	// Format: projects/{project}/locations/{location}/workflows/{workflow}
	// Example: projects/myabc-123/locations/europe-west1/workflows/my-lro
	name string
	// Project in which Workflow is deployed, for example myproject-123
	Project string
	// Location of workflow, for example: europe-west1
	Location string
	// Workflow name, for example: my-lro-workflow
	Workflow string
}

// SpannerConfig is used to configure the underlygin Google Cloud Spanner client.
type SpannerConfig struct {
	// Project
	Project string
	// Spanner Instance
	Instance string
	// Spanner Database
	Database string
	// The name of the Spanner table used to keep track of LROs
	Table string
	// Database role
	Role string
}

// Operation is the object used to manage the relevant LROs activties.
type Operation[T Checkpoint] struct {
	ctx       context.Context
	client    *Client
	id        string
	operation *longrunningpb.Operation
}

// A Checkpoint object of any type.
type Checkpoint interface{}

/*
NewClient creates a new lro Client object. The function takes five arguments:
  - ctx: The Context
  - spannerConfig: The configuration to setup the underlying Google Spanner client
  - workflowsConfig: The (optional) configuration to setup the underlyging Google Cloud Workflows client
*/
func NewClient(ctx context.Context, spannerConfig *SpannerConfig, workflowsConfig *WorkflowsConfig) (*Client, error) {
	// Create a new Client object
	client := &Client{
		spannerConfig: spannerConfig,
	}

	if spannerConfig != nil {
		client.spannerConfig = spannerConfig
		// Establish sproto spanner connection with fine grained table-level role
		c, err := sproto.NewClient(ctx, spannerConfig.Project, spannerConfig.Instance, spannerConfig.Database, spannerConfig.Role)
		if err != nil {
			return nil, err
		}
		client.spanner = c
	} else {
		return nil, fmt.Errorf("spannerConfig is required but not provided")
	}

	// Instantiate a new Workflows client if provided
	if workflowsConfig != nil {
		workflowsConfig.name = fmt.Sprintf("projects/%s/locations/%s/workflows/%s",
			workflowsConfig.Project, workflowsConfig.Location, workflowsConfig.Workflow)
		client.WorkflowsConfig = workflowsConfig
		c, err := executions.NewClient(ctx)
		if err != nil {
			return nil, err
		}
		client.workflows = c
	}

	return client, nil
}

/*
Close closes the underlying spanner.Client instance.
*/
func (c *Client) Close() {
	c.spanner.Close()
}

// getOperation is an internal method use to get a specified operation.
func (c *Client) GetOperation(ctx context.Context, operation string) (*longrunningpb.Operation, error) {
	// validate arguments
	err := validate.Argument("name", operation, validate.OperationRegex)
	if err != nil {
		return nil, err
	}

	// read operation resource from spanner
	op := &longrunningpb.Operation{}
	err = c.spanner.ReadProto(ctx, c.spannerConfig.Table, spanner.Key{operation}, OperationColumnName, op, nil)
	if err != nil {
		if _, ok := err.(sproto.ErrNotFound); ok {
			// Handle the ErrNotFound case.
			return nil, ErrNotFound{
				Operation: operation,
			}
		} else {
			// Handle other error types.
			return nil, fmt.Errorf("read operation from database: %w", err)
		}
	}

	return op, nil
}

// Wait polls the provided operation and waits until done.
func (c *Client) Wait(ctx context.Context, operation string, timeout time.Duration) (*longrunningpb.Operation, error) {
	// Set the default timeout
	if timeout == 0 {
		timeout = time.Second * 77
	}
	startTime := time.Now()

	// start loop to check if operation is done or timeout has passed
	var op *longrunningpb.Operation
	var err error
	for {
		op, err = c.GetOperation(ctx, operation)
		if err != nil {
			return nil, err
		}
		if op.Done {
			return op, nil
		}

		timePassed := time.Since(startTime)
		if timePassed.Seconds() > timeout.Seconds() {
			return nil, ErrWaitDeadlineExceeded{timeout: timeout}
		}
		time.Sleep(777 * time.Millisecond)
	}
}

// BatchWait is a batch version of the WaitOperation method.
// Takes three agruments:
//   - ctx: The Context header
//   - operations: An array of LRO names, for example: []string{"operations/123", "operations/456", ...}
//   - timeoute: the timeout duration to apply with each operation
func (c *Client) BatchWait(ctx context.Context, operations []string, timeout time.Duration) ([]*longrunningpb.Operation, error) {
	// iterate through the requests
	errs, ctx := errgroup.WithContext(ctx)
	results := make([]*longrunningpb.Operation, len(operations))
	for i, operation := range operations {
		i := i
		errs.Go(func() error {
			op, err := c.Wait(ctx, operation, timeout)
			if err != nil {
				return err
			}
			results[i] = op

			return nil
		})
		// Add some spacing between the api hits.
		time.Sleep(time.Millisecond * 77)
	}

	err := errs.Wait()
	if err != nil {
		return nil, err
	}

	return results, nil
}

// create stores a new long-running operation in spanner, with done=false
func (o *Operation[T]) create() error {
	// create new unpopulated long-running operation
	o.operation = &longrunningpb.Operation{}

	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	o.id = id.String()
	o.operation.Name = "operations/" + id.String()

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": o.operation}
	err = o.client.spanner.InsertRow(o.ctx, o.client.spannerConfig.Table, row)
	if err != nil {
		return err
	}

	return nil
}

// Get returns a long-running operation
func (o *Operation[T]) Get() (*longrunningpb.Operation, error) {
	return o.client.GetOperation(o.ctx, "operations/"+o.id)
}

// Delete deletes the LRO (including )
func (o *Operation[T]) Delete() (*emptypb.Empty, error) {
	// validate operation name
	err := validate.Argument("name", "operations/"+o.id, validate.OperationRegex)
	if err != nil {
		return nil, err
	}

	// validate existence of operation
	_, err = o.Get()
	if err != nil {
		return nil, err
	}

	// delete operation
	err = o.client.spanner.DeleteRow(o.ctx, o.client.spannerConfig.Table, spanner.Key{"operations/" + o.id})
	if err != nil {
		return nil, fmt.Errorf("delete operation (operations/%s): %w", o.id, err)
	}
	return &emptypb.Empty{}, nil
}

// SetSuccessful updates an existing long-running operation's done field to true, sets the response and updates the
// metadata if provided.
func (o *Operation[T]) Done(response proto.Message) (*longrunningpb.Operation, error) {
	// get operation
	op, err := o.Get()
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

	// update in spanner by first deleting and then writing
	_, err = o.Delete()
	if err != nil {
		return nil, err
	}

	//  write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = o.client.spanner.InsertRow(o.ctx, o.client.spannerConfig.Table, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// Error updates an existing long-running operation's done field to true.
func (o *Operation[T]) Error(error error) (*longrunningpb.Operation, error) {
	// get operation
	op, err := o.Get()
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

	// update in spanner by first deleting and then writing
	_, err = o.Delete()
	if err != nil {
		return nil, err
	}

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = o.client.spanner.InsertRow(o.ctx, o.client.spannerConfig.Table, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

/*
WaitAndResume orchestrates the pausing and resumption of an LRO using a Google Cloud Workflow.

This function performs the following steps:

 1. Saves the current checkpoint for the operation.
 2. Triggers a Google Cloud Workflow that polls the provided list of operations.
 3. Once all operations are complete, the workflow re-invokes the same method,
    passing a special header `x-alis-operation-id` to resume the original business logic using the saved checkpoint.

Parameters:
  - operations: A slice of operation IDs to monitor.
  - timeout: The overal time out for the polling.
  - checkpoint: The current state data to be saved for later resumption.
  - method: The method which will be resumed after the operations are done.
    for example: krynauws.pl.lros.v1.ReviewService/RequestReview

Returns:
  - An error if saving the checkpoint or initiating the workflow fails, otherwise nil.
*/
func (o *Operation[T]) WaitAndResume(operations []string, timeout time.Duration, checkpoint *T, method string) (*longrunningpb.Operation, error) {
	if o.client.WorkflowsConfig == nil {
		return nil, fmt.Errorf("the Google Cloud Workflow config is not setup with the client instantiation, please add the WorkflowsConfig to the NewClient method call ")
	}

	// First it saves the checkpoint
	err := o.SaveCheckpoint("operations/"+o.id, checkpoint)
	if err != nil {
		return nil, err
	}

	// Prepare the Google Cloud Workflow arguments
	type Args struct {
		OperationId string   // The LRO Id of the main method.
		Operations  []string // The list of operations to wait for.
		Method      string
		Timeout     float64
	}
	// Configure the arguments to pass into the container at runtime.
	// The Workflow service requires the argument in JSON format.
	args, err := json.Marshal(Args{
		OperationId: o.id,
		Operations:  operations,
		Method:      method,
		Timeout:     timeout.Seconds(),
	})
	if err != nil {
		return nil, err
	}
	// Launch Google Cloud Workflows and wait...
	_, err = o.client.workflows.CreateExecution(o.ctx, &executionspb.CreateExecutionRequest{
		Parent: o.client.WorkflowsConfig.name,
		Execution: &executionspb.Execution{
			Argument:     string(args),
			CallLogLevel: executionspb.Execution_LOG_ALL_CALLS,
		},
	})
	if err != nil {
		return nil, err
	}

	// Get a copy of the LRO
	op, err := o.Get()
	if err != nil {
		return nil, err
	}

	return op, nil
}

// UpdateMetadata updates an existing long-running operation's metadata.  Metadata typically
// contains progress information and common metadata such as create time.
func (o *Operation[T]) SetMetadata(metadata proto.Message) (*longrunningpb.Operation, error) {
	// get operation
	op, err := o.Get()
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
	_, err = o.Delete()
	if err != nil {
		return nil, err
	}

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = o.client.spanner.InsertRow(o.ctx, o.client.spannerConfig.Table, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// SaveCheckpoint saves a current checkpoint with the LRO resource
func (o *Operation[T]) SaveCheckpoint(operation string, checkpoint *T) error {
	buffer := bytes.Buffer{}
	enc := gob.NewEncoder(&buffer)
	if err := enc.Encode(checkpoint); err != nil {
		return err
	}

	err := o.client.spanner.UpdateRow(o.ctx, o.client.spannerConfig.Table, map[string]interface{}{
		"key":        operation,
		"Checkpoint": buffer.Bytes(),
	})
	if err != nil {
		return err
	}

	return nil
}

// LoadCheckpoint loads the current checkpoint stored with the LRO resource
func (o *Operation[T]) LoadCheckpoint() (*T, error) {
	var data T
	row, err := o.client.spanner.ReadRow(o.ctx, o.client.spannerConfig.Table, spanner.Key{"operations/" + o.id}, []string{CheckpointColumnName}, nil)
	if err != nil {
		return &data, err
	}
	if row[CheckpointColumnName] != nil {

		checkpointString, ok := row[CheckpointColumnName].(string)
		if !ok {
			return &data, fmt.Errorf("checkpoint data is not string")
		}

		// Decode from base643
		decodedData, err := base64.StdEncoding.DecodeString(checkpointString)
		if err != nil {
			fmt.Println("Error decoding:", err)
			return &data, err
		}

		var buffer bytes.Buffer
		buffer.Write(decodedData)
		decoder := gob.NewDecoder(&buffer)

		err = decoder.Decode(&data)
		if err != nil {
			return &data, err
		}
	}

	return &data, nil
}

// SetResponse retrieves the underlying LRO and unmarshals the Response into the provided response object.
// It takes three arguments
//   - ctx: Context
//   - operation: The resource name of the operation in the format `operations/*`
//   - response: The response object into which the underlyging response of the LRO should be marshalled into.
func (c *Client) UnmarshalOperation(ctx context.Context, operation string, response, metadata protoreflect.ProtoMessage) error {
	op, err := c.GetOperation(ctx, operation)
	if err != nil {
		return err
	}

	// Unmarshal the Response
	err = anypb.UnmarshalTo(op.GetResponse(), response, proto.UnmarshalOptions{})
	if err != nil {
		return err
	}

	// Unmarshal the Metadata
	err = anypb.UnmarshalTo(op.GetMetadata(), metadata, proto.UnmarshalOptions{})
	if err != nil {
		return err
	}
	return nil
}

/*
Init initializes a Long-Running Operation (LRO) with the provided Checkpoint type (T).

This function intelligently checks for an existing LRO by looking for the 'alis-x-operation-id' header.
If found, it reconnects to the existing operation; otherwise, it creates a new LRO.

Example (with Checkpoint):

	// CheckPoint is a custom object which will be stored alongside the LRO
	type CheckPoint struct {
			Next        string
			Review      string
			Rating      int32
			ApprovedBy  string
			ApprovalLRO string
	}
	var checkpoint *CheckPoint
	op, checkpoint, err := lro.Create[CheckPoint](ctx, lroClient)

Example (without a Checkpoint):

	op, _, err := lro.Create[interface{}](ctx, lroClient)
*/
func Init[T Checkpoint](ctx context.Context, client *Client) (op *Operation[T], checkpoint *T, err error) {
	op = &Operation[T]{
		ctx:    context.WithoutCancel(ctx),
		client: client,
	}

	// In order to handle the resumable LRO design pattern, we add the relevant headers to the outgoing context.
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if len(md.Get(OperationIdHeaderKey)) > 0 {
			// We found a special header x-alis-operation-id, it suggest that the LRO is an existing one.
			// No need to create one
			op.id = "operations/" + md.Get(OperationIdHeaderKey)[0]

			checkpoint, err = op.LoadCheckpoint()
			if err != nil {
				return nil, nil, err
			}
			return op, checkpoint, nil
		} else {
			err = op.create()
			if err != nil {
				return nil, nil, err
			}
		}
	} else {
		// Handle the scenario where no context exists.
		err = op.create()
		if err != nil {
			return nil, nil, err
		}
	}

	err = op.create()
	if err != nil {
		return nil, nil, err
	}

	return op, nil, err
}
