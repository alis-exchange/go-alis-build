package lro

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/spanner"
	"cloud.google.com/go/workflows/executions/apiv1/executionspb"
	"github.com/google/uuid"
	"go.alis.build/lro/internal/validate"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	// OperationIdHeaderKey is use to indicate the the LRO already exists, and does not need to be created
	OperationIdHeaderKey = "x-alis-operation-id"
)

// ResumableOperation is the object used to manage the relevant LROs activties.
type ResumableOperation[T Checkpoint] struct {
	ctx            context.Context
	client         *Client
	id             string
	operation      *longrunningpb.Operation
	resumeEndpoint string
	devMode        bool
}

// A Checkpoint object of any type.
type Checkpoint interface{}

/*
NewResumableOperation initialises a Long-Running Operation (LRO) with the provided Checkpoint type (T).

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
	op, checkpoint, err := lro.Create[CheckPoint](ctx, lroClient, "https://...../RequestReview")

Example (without a Checkpoint):

	op, _, err := lro.Create[interface{}](ctx, lroClient)
*/
func NewResumableOperation[T Checkpoint](ctx context.Context, client *Client, resumeEndpoint string) (op *ResumableOperation[T], checkpoint *T, err error) {
	op = &ResumableOperation[T]{
		ctx:            context.WithoutCancel(ctx),
		client:         client,
		resumeEndpoint: resumeEndpoint,
	}

	// In order to handle the resumable LRO design pattern, we add the relevant headers to the outgoing context.
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if len(md.Get(OperationIdHeaderKey)) > 0 {
			// We found a special header x-alis-operation-id, it suggest that the LRO is an existing one.
			// No need to create one
			op.id = md.Get(OperationIdHeaderKey)[0]

			checkpoint, err = op.loadCheckpoint()
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

// SetLocal could be used to switch off any resumable features and should only be used for local testing purposes
func (o *ResumableOperation[T]) ActivateDevMode() {
	o.devMode = true
}

// DevMode return 'true' if the LRO is running in development mode used for local testing purposes
func (o *ResumableOperation[T]) DevMode() bool {
	return o.devMode
}

// create stores a new long-running operation in spanner, with done=false
func (o *ResumableOperation[T]) create() error {
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
func (o *ResumableOperation[T]) Get() (*longrunningpb.Operation, error) {
	return o.client.Get(o.ctx, "operations/"+o.id)
}

// Delete deletes the LRO (including )
func (o *ResumableOperation[T]) Delete() (*emptypb.Empty, error) {
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
func (o *ResumableOperation[T]) Done(response proto.Message) error {
	// get operation
	op, err := o.Get()
	if err != nil {
		return err
	}

	// update done and result
	op.Done = true
	if response != nil {
		resultAny, err := anypb.New(response)
		if err != nil {
			return err
		}
		op.Result = &longrunningpb.Operation_Response{Response: resultAny}
	}

	//  write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = o.client.spanner.UpsertRow(o.ctx, o.client.spannerConfig.Table, row)
	if err != nil {
		return err
	}

	return nil
}

// Error updates an existing long-running operation's done field to true.
func (o *ResumableOperation[T]) Error(error error) error {
	// get operation
	op, err := o.Get()
	if err != nil {
		return err
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

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = o.client.spanner.UpsertRow(o.ctx, o.client.spannerConfig.Table, row)
	if err != nil {
		return err
	}

	return nil
}

/*
WaitAsync orchestrates the pausing and resumption of an LRO using a Google Cloud Workflow.

This function performs the following steps:

 1. Saves the current checkpoint for the operation.
 2. Triggers a Google Cloud Workflow that polls the provided list of operations.
 3. Once all operations are complete, the workflow re-invokes the same method,
    passing a special header `x-alis-operation-id` to resume the original business logic using the saved checkpoint.

Parameters:
  - operations: A slice of operation IDs to monitor.
  - timeout: The overal time out for the polling.
  - pollFrequency: How oftern the LRO needs to be polled for status
  - checkpoint: The current state data to be saved for later resumption.
  - pollEndpoint: The RESTFull endpoint used for polling the status of the provided LROs
    for example: https://.../.../GetOperation
*/
func (o *ResumableOperation[T]) WaitAsync(operations []string, timeout, pollFrequency time.Duration, pollEndpoint string, checkpoint *T) error {
	if o.devMode {
		return fmt.Errorf("unable to run WaitAsync while in development mode")
	}

	if o.client.workflowsConfig == nil {
		return fmt.Errorf("the Google Cloud Workflow config is not setup with the client instantiation, please add the WorkflowsConfig to the NewClient method call ")
	}

	// First it saves the checkpoint
	err := o.saveCheckpoint("operations/"+o.id, checkpoint)
	if err != nil {
		return err
	}

	// Prepare the Google Cloud Workflow argument
	type Args struct {
		OperationId            string   `json:"operationId"`
		Operations             []string `json:"operations"`
		Timeout                int64    `json:"timeout"`
		PollFrequency          int64    `json:"pollFrequency"`
		PollEndpoint           string   `json:"pollEndpoint"`
		PollEndpointAudience   string   `json:"pollEndpointAudience"`
		ResumeEndpoint         string   `json:"resumeEndpoint"`
		ResumeEndpointAudience string   `json:"resumeEndpointAudience"`
	}

	// Retrieve the Audience values required by the authenticated api call made by the workflow.
	pollUrl, err := url.Parse(pollEndpoint)
	if err != nil {
		return fmt.Errorf("invalid polling endpoint (%s): %w", pollEndpoint, err)
	}
	resumeUrl, err := url.Parse(o.resumeEndpoint)
	if err != nil {
		return fmt.Errorf("invalid resume endpoint (%s): %w", o.resumeEndpoint, err)
	}

	// Configure the arguments to pass into the container at runtime.
	// The Workflow service requires the argument in JSON format.
	args, err := json.Marshal(Args{
		OperationId:            o.id,
		Operations:             operations,
		Timeout:                int64(timeout.Seconds()),
		PollFrequency:          int64(pollFrequency.Seconds()),
		PollEndpoint:           pollEndpoint,
		PollEndpointAudience:   "https://" + pollUrl.Host,
		ResumeEndpoint:         o.resumeEndpoint,
		ResumeEndpointAudience: "https://" + resumeUrl.Host,
	})
	if err != nil {
		return err
	}
	// Launch Google Cloud Workflows and wait...
	_, err = o.client.workflows.CreateExecution(o.ctx, &executionspb.CreateExecutionRequest{
		Parent: o.client.workflowsConfig.name,
		Execution: &executionspb.Execution{
			Argument:     string(args),
			CallLogLevel: executionspb.Execution_LOG_ALL_CALLS,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// Wait polls the provided operation and waits until done.
// The underlying polling is using the same client configured for the ResumableOpertion instance.  If a different client is required
// for polloing use the lro.Wait() method instead.
func (o *ResumableOperation[T]) Wait(ctx context.Context, operation string, timeout time.Duration) (*longrunningpb.Operation, error) {
	// Set the default timeout
	if timeout == 0 {
		timeout = time.Second * 77
	}
	startTime := time.Now()

	// start loop to check if operation is done or timeout has passed
	var op *longrunningpb.Operation
	var err error
	for {
		op, err = o.client.Get(o.ctx, operation)
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

// UpdateMetadata updates an existing long-running operation's metadata.  Metadata typically
// contains progress information and common metadata such as create time.
func (o *ResumableOperation[T]) SetMetadata(metadata proto.Message) (*longrunningpb.Operation, error) {
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

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = o.client.spanner.UpsertRow(o.ctx, o.client.spannerConfig.Table, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// SaveCheckpoint saves a current checkpoint with the LRO resource
func (o *ResumableOperation[T]) saveCheckpoint(operation string, checkpoint *T) error {
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
func (o *ResumableOperation[T]) loadCheckpoint() (*T, error) {
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
