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
	"golang.org/x/sync/errgroup"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"

	"go.alis.build/lro/internal/validate"
)

const (
	// OperationIdHeaderKey is use to indicate the the LRO already exists, and does not need to be created
	OperationIdHeaderKey = "x-alis-operation-id"
)

// ResumableOptions are used to configure options with the ResumableOperation object.
type ResumableOptions struct {
	ResumeEndpoint string
	Host           string
}

// ResumableOption is a functional option for the NewResumableOperation method.
type ResumableOption func(*ResumableOptions)

// WithResumeEndpoint sets the resume endpoint for the resumable operation.
// If not provided, NewResumableOperation will infer the resume endpoint from the Host option and the context.
func WithResumeEndpoint(endpoint string) ResumableOption {
	return func(opts *ResumableOptions) {
		opts.ResumeEndpoint = endpoint
	}
}

// WithHost sets the host for the resumable operation.
// This is used to infer the resume endpoint if not provided.
func WithHost(host string) ResumableOption {
	return func(opts *ResumableOptions) {
		opts.Host = host
	}
}

// WaitOptions manages additional functionality with the relevant Wait methods.
type WaitOptions struct {
	Client               *Client
	Timeout              time.Duration
	PollFrequency        time.Duration
	PollEndpoint         string
	InitialSleepDuration time.Duration
}

// WaitOption is a functional option for the relevant Wait methods.
type WaitOption func(*WaitOptions)

// WithClient specifies a custom Client for polling the Long-Running Operation (LRO).
// Use this when the polling client differs from the one used for the initial operation.
func WithClient(client *Client) WaitOption {
	return func(opts *WaitOptions) {
		opts.Client = client
	}
}

/*
WithTimeout sets the Timeout for polling the underlying LRO.

Timeout defaults to 7 minutes if not provided.
*/
func WithTimeout(timeout time.Duration) WaitOption {
	return func(opts *WaitOptions) {
		opts.Timeout = timeout
	}
}

// WithPollFrequency configures how often the operation checks for completion.
// For time-critical tasks, use a lower value (e.g., 7 seconds).
// Note: Frequent polling increases API calls, potentially impacting costs.
// Defaults to 7 seconds if not provided.
func WithPollFrequency(pollFrequency time.Duration) WaitOption {
	return func(opts *WaitOptions) {
		opts.PollFrequency = pollFrequency
	}
}

// WithPollEndpoint sets the specific REST API endpoint for Google Cloud Workflows to poll
// when checking the Long-Running Operation(s) status.
func WithPollEndpoint(pollEndpoint string) WaitOption {
	return func(opts *WaitOptions) {
		opts.PollEndpoint = pollEndpoint
	}
}

// WithInitialSleep sets the sleep duration to be incurred in the first step of Google Cloud Workflow.
func WithInitialSleepDuration(initialSleepDuration time.Duration) WaitOption {
	return func(opts *WaitOptions) {
		opts.InitialSleepDuration = initialSleepDuration
	}
}

// ResumableOperation is the object used to manage the relevant LROs activties.
type ResumableOperation[T State] struct {
	ctx            context.Context
	client         *Client
	id             string
	operation      *longrunningpb.Operation
	resumeEndpoint string
	devMode        bool
}

// A State object of any type.
type State interface{}

/*
NewResumableOperation initialises a Long-Running Operation (LRO) with the provided State type (T).

This function intelligently checks for an existing LRO by looking for the 'alis-x-operation-id' header.
If found, it reconnects to the existing operation; otherwise, it creates a new LRO.

Example (with State):

	// State is a custom object which will be stored alongside the LRO
	type State struct {
			Next        string
			Review      string
			Rating      int32
			ApprovedBy  string
			ApprovalLRO string
	}
	var state *State
	op, state, err := lro.Create[State](ctx, lroClient, lro.WithHost("https://.....a.run.app"))
*/
func NewResumableOperation[T State](ctx context.Context, client *Client, opts ...ResumableOption) (op *ResumableOperation[T], state *T, err error) {
	options := &ResumableOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Get the method from the context
	method, ok := grpc.Method(ctx)
	// Construct the resume endpoint from the host and method
	var resumeEndpoint string
	if options.Host != "" && ok && method != "" {
		resumeEndpoint = options.Host + method
	}
	// If the resume endpoint is provided, use it
	if options.ResumeEndpoint != "" {
		resumeEndpoint = options.ResumeEndpoint
	}

	op = &ResumableOperation[T]{
		ctx:            context.WithoutCancel(ctx),
		client:         client,
		resumeEndpoint: resumeEndpoint,
	}

	// In order to handle the resumable LRO design pattern, we add the relevant headers to the outgoing context.
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if len(md.Get(OperationIdHeaderKey)) > 0 {
			// We found a special header x-alis-operation-id, it suggests that the LRO is an existing one.
			// No need to create one
			op.id = md.Get(OperationIdHeaderKey)[0]

			state, err = op.loadState()
			if err != nil {
				return nil, nil, err
			}
			return op, state, nil
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

	return op, new(T), err
}

/*
ActivateDevMode could be used to switch off any resumable features and should only be used for local testing purposes.

For example:

	// Enable Development mode
	op.ActivateDevMode()
*/
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

// ReturnRPC is mostly used by gRPC methods required to return gRPC compliant error codes.
func (o *ResumableOperation[T]) ReturnRPC() (*longrunningpb.Operation, error) {
	op, err := o.client.Get(o.ctx, "operations/"+o.id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "retrieve operation: %s", err)
	}
	return op, nil
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

 1. Saves the current state for the operation.
 2. Triggers a Google Cloud Workflow that polls the provided list of operations. Waiting can be performed independent of operations by setting initialSleep parameter to the number of seconds to wait before resuming.
 3. Once all operations are complete, the workflow re-invokes the same method,
    passing a special header `x-alis-operation-id` to resume the original business logic using the saved state.

Parameters:
  - operations: A slice of operation IDs to monitor.
  - state: The current state data to be saved for later resumption.
  - pollEndpoint: The RESTFull endpoint used for polling the status of the provided LROs
    for example: https://.../.../GetOperation
*/
func (o *ResumableOperation[T]) WaitAsync(operations []string, state *T, opts ...WaitOption) error {
	// Add all the provided options to the WaitOptions object
	options := &WaitOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if o.devMode {
		return fmt.Errorf("unable to run WaitAsync while in development mode")
	}

	if o.client.workflowsConfig == nil {
		return fmt.Errorf("the Google Cloud Workflow config is not setup with the client instantiation, please add the WorkflowsConfig to the NewClient method call ")
	}

	// First it saves the state
	err := o.saveState("operations/"+o.id, state)
	if err != nil {
		return err
	}

	// Ensure operations is not nil, needs to be an array for Google Cloud Workflows
	if operations == nil {
		operations = []string{}
	}

	// Set the Timeout
	var timeout time.Duration
	if options.Timeout != 0 {
		timeout = options.Timeout
	} else {
		timeout = 7 * time.Minute
	}

	// Set the Polling frequency
	var pollFrequency time.Duration
	if options.PollFrequency != 0 {
		pollFrequency = options.PollFrequency
	} else {
		pollFrequency = time.Second * 7
	}

	// Set the initial sleep duration
	var initialSleep time.Duration
	if options.InitialSleepDuration != 0 {
		initialSleep = options.InitialSleepDuration
	} else {
		initialSleep = time.Second * 0
	}

	// Set the Poll Endpoint
	// Only required if waiting with Google Workflow
	var pollEndpoint string
	if options.PollEndpoint != "" {
		pollEndpoint = options.PollEndpoint
	} else {
		return fmt.Errorf("polling endpoint is required but not specified, please add the lro.WithPollEndpoint() option to your WaitAsync method")
	}

	// Prepare the Google Cloud Workflow argument
	type Args struct {
		OperationId            string   `json:"operationId"`
		Operations             []string `json:"operations"`
		Timeout                int64    `json:"timeout"`
		InitialSleepDuration   int64    `json:"initialSleepDuration"`
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
		InitialSleepDuration:   int64(initialSleep.Seconds()),
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

/*
Wait blocks until the provided long-running operation completes or times out.

Optional configuration using 'opts':

  - InitialSleep: Minimum duration to wait irrespective of operation waiting.
  - Timeout: Maximum duration to wait (default: 7 minutes).
  - PollFrequency: How often to check operation status (default: 7 seconds).
  - Client: Custom client for polling (default: client used to create ResumableOperation).

Returns the final operation status or an error if timeout is exceeded or another issue occurs.
*/
func (o *ResumableOperation[T]) Wait(operations []string, state *T, resumeCallback func(string), opts ...WaitOption) error {
	var err error

	// Add all the provided options to the WaitOptions object
	options := &WaitOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// First save the state
	err = o.saveState("operations/"+o.id, state)
	if err != nil {
		return err
	}

	// Use the specified Client if provided
	var client *Client
	if options.Client != nil {
		client = options.Client
	} else {
		// use the client specified with NewResumableOption
		client = o.client
	}

	// Set the Timeout
	var timeout time.Duration
	if options.Timeout != 0 {
		timeout = options.Timeout
	} else {
		timeout = 7 * time.Minute
	}

	// Set the initial sleep duration
	var initialSleep time.Duration
	if options.InitialSleepDuration != 0 {
		initialSleep = options.InitialSleepDuration
	} else {
		initialSleep = time.Second * 0
	}

	// Set the Polling frequency
	var pollFrequency time.Duration
	if options.PollFrequency != 0 {
		pollFrequency = options.PollFrequency
	} else {
		pollFrequency = time.Second * 7
	}

	startTime := time.Now()

	// if set, incur the initial sleep
	// operations are still processing in this time therefore startTime has already been measured
	time.Sleep(initialSleep)

	// The following block of code is only applicable when waiting for operations
	{
		g := new(errgroup.Group)

		// locally wait for each operation and contribute status to wait group
		for _, op := range operations {
			g.Go(func() error {
				// start loop to check if operation is done or timeout has passed
				var operation *longrunningpb.Operation
				for {
					operation, err = client.Get(o.ctx, op)
					if err != nil {
						return err
					}
					if operation.Done {
						return nil
					}

					timePassed := time.Since(startTime)
					if timePassed.Seconds() > timeout.Seconds() {
						return ErrWaitDeadlineExceeded{
							message: fmt.Sprintf("operation (%s) exceeded timeout deadline of %0.0f seconds",
								op, timeout.Seconds()),
						}
					}
					time.Sleep(pollFrequency)
				}
			})
		}
		groupErr := g.Wait()
		if groupErr != nil {
			return groupErr
		}
	}

	// An optional waiting callback can be provided if the user would like to resume execution upon completion of waiting
	if resumeCallback != nil {
		resumeCallback(o.id)
	}

	return nil
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

// SaveState saves a current state with the LRO resource
func (o *ResumableOperation[T]) saveState(operation string, state *T) error {
	buffer := bytes.Buffer{}
	enc := gob.NewEncoder(&buffer)
	if err := enc.Encode(state); err != nil {
		return err
	}

	err := o.client.spanner.UpdateRow(o.ctx, o.client.spannerConfig.Table, map[string]interface{}{
		"key":   operation,
		"State": buffer.Bytes(),
	})
	if err != nil {
		return err
	}

	return nil
}

// loadState loads the current state stored with the LRO resource
func (o *ResumableOperation[T]) loadState() (*T, error) {
	var data T
	row, err := o.client.spanner.ReadRow(o.ctx, o.client.spannerConfig.Table, spanner.Key{"operations/" + o.id}, []string{StateColumnName}, nil)
	if err != nil {
		return &data, err
	}
	if row[StateColumnName] != nil {

		stateString, ok := row[StateColumnName].(string)
		if !ok {
			return &data, fmt.Errorf("state data is not string")
		}

		// Decode from base643
		decodedData, err := base64.StdEncoding.DecodeString(stateString)
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
