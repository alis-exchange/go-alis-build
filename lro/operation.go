package lro

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
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
)

// OperationIdHeaderKey is use to indicate the the LRO already exists, and does not need to be created
const OperationIdHeaderKey = "x-alis-operation-id"

// Operation is an object to to manage the lifecycle of a Google Longrunning-Operation.
type Operation[T any] struct {
	ctx    context.Context
	client *Client
	// The Operation resource name
	name string
	// The custom State object used by the main business logic to transfer state between async wait operations
	state *T
	// The label that could be used in the main business logic with 'goto' or 'switch' statements
	resumePoint string
	// The fully qualified method name to be resumed
	// For example: "/myorg.co.jobs.v1.JobsService/GenerateClientReports"
	resumeMethod string
	// LocalResumeCallback is used for local testing
	asyncCallbackFn func(ctx context.Context)
	// Devmode enabled
	devMode bool
}

type OperationOptions struct {
	// The fully qualified method name to be resumed
	// For example: "/myorg.co.jobs.v1.JobsService/GenerateClientReports"
	resumeMethod string
	// The name of an existing Operation resource
	existingOperation string
	// LocalResumeCallback is used for local testing
	asyncCallbackFn func(ctx context.Context)
}

// ClientOption is a functional option for the NewOperation method.
type OperationOption func(*OperationOptions)

/*
WithResumeMethod sets the fully qualified method name to be resumed

In most scenarios this will not be required since the method name will
be automatically made avaialable by the grpc server exposed by the grpc.Method()
Example: "/myorg.co.jobs.v1.JobsService/GenerateClientReports"
*/
func WithResumeMethod(name string) OperationOption {
	return func(opts *OperationOptions) {
		opts.resumeMethod = name
	}
}

/*
WithExistingOperation allows one to instantiate a new Operation object from an
existing LRO. If this is provided the underlyging NewOperation method will
not create a new Operation resource nor infer one from the ctx.
*/
func WithExistingOperation(name string) OperationOption {
	return func(opts *OperationOptions) {
		opts.existingOperation = name
	}
}

/*
WithCallbackFn sets the function to call back when running an Async method locally.
*/
func WithCallbackFn(fn func(ctx context.Context)) OperationOption {
	return func(opts *OperationOptions) {
		opts.asyncCallbackFn = fn
	}
}

/*
NewOperation creates a new Operation object used to simplify the management of the underlying LRO.
The default behaviour of this function is to create a new underlying LRO.
However, If the provided ctx contains a value for the x-alis-operation-id key, the function instantiates an Operation to manage the LRO identified by the x-alis-operation-id.
The same applies if the WithExistingOperation option is used.

Example (non-resumable):

	// No need to create a custom State object, simply add 'any' type
	op, _ := lro.NewOperation[any](ctx, client)

Example (resumable lro with internal state):

	// In this case, define your own state object which could be used to transfer state between async wait operations.
	type MyState struct{}
	op, _ := lro.NewOperation[MyState](ctx, client)

	// Retrieve the state object.
	state = op.State()
*/
func NewOperation[T any](ctx context.Context, client *Client, opts ...OperationOption) (*Operation[T], error) {
	var err error
	// Configure the defualt options
	options := &OperationOptions{
		resumeMethod:      "",
		existingOperation: "",
		asyncCallbackFn:   func(ctx context.Context) {},
	}
	// Add the user provided overrides.
	for _, opt := range opts {
		opt(options)
	}

	// Construct an Operation object
	operation := &Operation[T]{
		ctx:             context.WithoutCancel(ctx),
		client:          client,
		name:            "",
		state:           new(T),
		resumePoint:     "",
		resumeMethod:    "",
		asyncCallbackFn: options.asyncCallbackFn,
		devMode:         false,
	}

	// Enable the devMode if not running on Cloud Run.
	if os.Getenv("K_SERVICE") == "" {
		operation.devMode = true
	}

	// Set the method if the user specified it, otherwise ry to get from the context.
	if options.resumeMethod != "" {
		operation.resumeMethod = options.resumeMethod
	} else {
		// Try to determine the method from the ctx
		methodName, ok := grpc.Method(operation.ctx)
		if ok {
			operation.resumeMethod = methodName
		}
	}

	// If the user specified an existing Operation, use this, otherwise infer from ctx.
	if options.existingOperation != "" {
		operation.name = options.existingOperation
	} else {
		// If in prod mode, try to get from the gRPC server
		if !operation.devMode {

			// Extract operation ID from incoming context metadata to support resumable long-running operations (LROs).
			// If x-alis-operation-id header exists, this is a continuation of an existing operation. In this case,
			// construct the operation name and remove the header to prevent unintended propagation in the local context.
			md, ok := metadata.FromIncomingContext(ctx)
			if ok {
				if len(md.Get(OperationIdHeaderKey)) > 0 {
					// We found a special header x-alis-operation-id, it suggests that the LRO is an existing one.
					operation.name = "operations/" + md.Get(OperationIdHeaderKey)[0]

					// Now that we 'used' the header key, let's remove it.
					md.Delete(OperationIdHeaderKey)
				}
			}
		} else {
			// in dev mode, the operation may be in the local ctx value passed in by the provided callback function.
			operationId, ok := ctx.Value(OperationIdHeaderKey).(string)
			if ok {
				operation.name = "operations/" + operationId
			}
		}
	}

	// By now, if the Operation name is not set, it suggest that this is a new Operation, lets create one.
	if operation.name == "" {
		// create new unpopulated long-running operation
		id, err := uuid.NewRandom()
		if err != nil {
			return nil, err
		}
		op := &longrunningpb.Operation{
			Name: "operations/" + id.String(),
		}
		operation.name = op.GetName()

		// add operation to database.
		_, err = operation.client.spanner.Apply(ctx, []*spanner.Mutation{
			spanner.Insert(operation.client.spannerTable, []string{"Operation"}, []any{op}),
		})
		if err != nil {
			return nil, err
		}
	} else {
		// The operation exists, get the details from the Spanner database.
		// No need to actually retrieve the Operation data from the database, only need the State and ResumePoint details, if available
		// row, err := operation.client.spanner.ReadRow(operation.ctx, operation.client.spannerTable,
		// 	spanner.Key{operation.name}, []string{StateColumnName, ResumePointColumnName}, nil)
		// if err != nil {
		// 	return nil, fmt.Errorf("read operation data from database: %w", err)
		// }

		spannerRow, err := operation.client.spanner.Single().ReadRow(operation.ctx, operation.client.spannerTable,
			spanner.Key{operation.name}, []string{StateColumnName, ResumePointColumnName})
		if err != nil {
			return nil, err
		}
		row := make(map[string]interface{})
		for i, columnName := range spannerRow.ColumnNames() {
			columnValue := spannerRow.ColumnValue(i)
			row[columnName] = parseStructPbValue(columnValue)
		}

		// Populate the State if available.
		if row[StateColumnName] != nil {
			// If the state is not of type any, we need to decode the state data from the database.
			// Decode from base64
			stateString, ok := row[StateColumnName].(string)
			if !ok {
				return nil, fmt.Errorf("state data is not string")
			}

			// Decode from base64
			decodedData, err := base64.StdEncoding.DecodeString(stateString)
			if err != nil {
				return nil, fmt.Errorf("decode state string data: %w", err)
			}

			var buffer bytes.Buffer
			buffer.Write(decodedData)
			decoder := gob.NewDecoder(&buffer)

			// We'll try to populate the state, but fail softly if unable to.
			err = decoder.Decode(&operation.state)
			if err != nil {
				// return nil, fmt.Errorf("decode state: %w", err)
			}
		}

		// Populate the Checkpoint if avaialable.
		if row[ResumePointColumnName] != nil {
			var ok bool
			operation.resumePoint, ok = row[ResumePointColumnName].(string)
			if !ok {
				return nil, fmt.Errorf("resumePoint data is not string")
			}
		}
	}

	return operation, err
}

// Name returns the name of the underlying Operation resource.
func (o *Operation[T]) Name() string {
	return o.name
}

// GetOperation retrieves the underlying longrunningpb.Operation.
func (o *Operation[T]) GetOperation() (*longrunningpb.Operation, error) {
	if o.name == "" {
		return nil, fmt.Errorf("operation name is nil")
	}
	return o.client.GetOperation(o.ctx, &longrunningpb.GetOperationRequest{Name: o.name})
}

func (o *Operation[T]) SetResumePoint(resumePoint string) {
	o.resumePoint = resumePoint
}

// ReturnRPC returns the underlying longrunningpb.Operation for further processing or sending to other APIs.
func (o *Operation[T]) ReturnRPC() (*longrunningpb.Operation, error) {
	op, err := o.client.GetOperation(o.ctx, &longrunningpb.GetOperationRequest{Name: o.name})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "retrieve operation: %s", err)
	}
	return op, nil
}

// Done marks the operation as done with a success response.
func (o *Operation[T]) Done(response proto.Message) error {
	// get operation
	op, err := o.GetOperation()
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

	// write operation to database.
	_, err = o.client.spanner.Apply(o.ctx, []*spanner.Mutation{
		spanner.InsertOrUpdate(o.client.spannerTable, []string{"Operation"}, []any{op}),
	})
	if err != nil {
		return err
	}

	return nil
}

// Error marks the operation as done with an error.
func (o *Operation[T]) Error(error error) error {
	// get operation
	op, err := o.GetOperation()
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

	// write operation to database.
	_, err = o.client.spanner.Apply(o.ctx, []*spanner.Mutation{
		spanner.InsertOrUpdate(o.client.spannerTable, []string{"Operation"}, []any{op}),
	})
	if err != nil {
		return err
	}

	return nil
}

// SetMetadata sets the metadata for the operation.
func (o *Operation[T]) SetMetadata(metadata proto.Message) (*longrunningpb.Operation, error) {
	// get operation
	op, err := o.GetOperation()
	if err != nil {
		return nil, err
	}

	// update metadata if required
	metaAny, err := anypb.New(metadata)
	if err != nil {
		return nil, err
	}
	op.Metadata = metaAny

	// write operation to database.
	_, err = o.client.spanner.Apply(o.ctx, []*spanner.Mutation{
		spanner.InsertOrUpdate(o.client.spannerTable, []string{"Operation"}, []any{op}),
	})
	if err != nil {
		return nil, err
	}

	return op, nil
}

// Delete deletes the LRO, including auxiliary columns
func (o *Operation[T]) Delete() error {
	// validate existence of operation
	_, err := o.GetOperation()
	if err != nil {
		return err
	}

	// delete operation
	_, err = o.client.spanner.Apply(o.ctx, []*spanner.Mutation{
		spanner.Delete(o.client.spannerTable, spanner.Key{o.name}),
	})
	if err != nil {
		return err
	}

	return nil
}

// SaveState saves a current state with the LRO resource
func (o *Operation[T]) SetState(state *T) {
	o.state = state
}

// ResumePoint retrieves the latest point from which to resume an asynchronous operation.
func (o *Operation[T]) ResumePoint() string {
	return o.resumePoint
}

// GetState returns the state of the current LRO.
func (o *Operation[T]) State() *T {
	return o.state
}

// InDevMode returns a true if the Operation is running locally
func (o *Operation[T]) InDevMode() bool {
	// We'll deactivate development mode if the service is running on Cloud Run
	return o.devMode
}

// WaitConfig is used to store the waiting configurations specified as functional WaitOption(s) in the Wait() and WaitAsync() methods.
type WaitConfig struct {
	// Standard Wait Configurations
	sleep         time.Duration // constant duration wait option
	timeout       time.Duration // overall timeout
	pollFrequency time.Duration // The interval duration which which to poll

	// Dependencies
	childOperations []string // The underlyging child Operations to wait for.

	// Wait for LROs from external Operations services
	service OperationsService

	// Async configurations
	asyncEnabled                   bool
	resumePoint                    string // Once the wait is complete, resume at this point.
	asyncChildGetOperationEndpoint string // The API endpoint which exposes a GetOperation method
}

// WaitOption is a functional option for WaitConfig.
type WaitOption func(*WaitConfig) error

// WithSleep specified a constant duration for which to wait.
func WithSleep(sleep time.Duration) WaitOption {
	return func(w *WaitConfig) error {
		w.sleep = sleep
		return nil
	}
}

// WithTimeout specified a constant duration after which the Wait method will return a ErrWaitDeadlineExceeded error.
func WithTimeout(timeout time.Duration) WaitOption {
	return func(w *WaitConfig) error {
		w.timeout = timeout
		return nil
	}
}

// WithPollFrequency specified a constant duration to use when polling the underlyging Child Operations to wait for.
func WithPollFrequency(pollFrequency time.Duration) WaitOption {
	return func(w *WaitConfig) error {
		w.pollFrequency = pollFrequency
		return nil
	}
}

// WithChildOperations specifies operations for which to wait.
// Format: ["operations/123", "operations/456", "operations/789"]
func WithChildOperations(operations ...string) WaitOption {
	return func(w *WaitConfig) error {
		if operations == nil {
			w.childOperations = []string{}
		} else {
			w.childOperations = operations
		}
		return nil
	}
}

// WithService allows one to override the underlying Operations client used to poll the child operations
func WithService(service OperationsService) WaitOption {
	return func(w *WaitConfig) error {
		w.service = service
		return nil
	}
}

// WithChildGetOperationEndpoint is used for local development
func WithChildGetOperationEndpoint(endpoint string) WaitOption {
	return func(w *WaitConfig) error {
		w.asyncChildGetOperationEndpoint = endpoint
		return nil
	}
}

// WithAsync allows one to exit the method and wait asynchronously.
func WithAsync(resumePoint string) WaitOption {
	return func(w *WaitConfig) error {
		w.asyncEnabled = true // explicitly communicate that the wait should run in async mode.
		w.resumePoint = resumePoint
		return nil
	}
}

/*
Wait blocks until the operation is marked as done or the configured
timeout is reached.

By default, Wait will wait for up to 7 minutes, polling every 3 seconds.
These values can be configured by providing [WaitOption].

A slice of child Operation names can be provided as a WaitOption.
In this case, Wait will block until the parent operation and all
child operations are marked as done, or until the timeout is
reached.

If the operation is not done when the timeout is reached,
Wait will return an [ErrWaitDeadlineExceeded] error.

	----

Example 1:

	// Simply wait for 30 seconds.
	op.Wait(WithSleep(30 * time.Second))

Example 2:

	// Make one or more hits to methods returning LROs, and wait for these to complete.
	op.Wait(WithChildOperations([]string{"operations/123", "operations/456"}))

Example 3:

	// Customise the poll frequency and timeout
	op.Wait(WithChildOperations([]string{"operations/123", "operations/456"}), WithTimeout(10*time.Minute), WithPollFrequency(30*time.Second))

Example 4:

	// If the underlying operations are from another product, you would need to use a different client to poll
	// the relevant GetOperation methods.  The service you connect to needs to satisfy the LRO interface as defined by google.longrunning package
	var conn grpc.ClientConnInterface // create a connection to the relevant gRPC server
	myLroClient := longrunningpb.NewOperationsClient(conn)
	op.Wait(WithChildOperations([]string{"operations/123", "operations/456"}), WithService(myLroClient))

Example 5:

	// If, however the operations from another product does not implement the google.longrunning service, you could use
	// any service that implements a GetOperation() method, therefore satisfying the [OperationsService] interface defined in this package.
	var myProductClient OperationsService
	op.Wait(WithChildOperations([]string{"operations/123", "operations/456"}), WithService(myProductClient))

Example 6:

	// Wait asynchronously for a longer time.
	op.SetState(&MyState{}) // Explicitly set the state before waiting asynchronously
	op.Wait(WithSleep(24*time.Hour), WithAsync("resumePoint1"))
	return nil  // a `return` statement usually follows when waiting asynchronously
*/
func (o *Operation[T]) Wait(opts ...WaitOption) error {
	// Set the default wait options.
	w := &WaitConfig{
		sleep:                          0,
		timeout:                        7 * time.Minute,
		pollFrequency:                  3 * time.Second,
		childOperations:                []string{},
		service:                        o.client,
		asyncEnabled:                   false,
		resumePoint:                    "",
		asyncChildGetOperationEndpoint: o.client.resumeHost + "/google.longrunning.Operations/GetOperation",
	}

	// Now override any values explicity configured with the WaitOptions.
	for _, opt := range opts {
		err := opt(w)
		// fail on error in option configuration
		if err != nil {
			return err
		}
	}

	// All options have been configures, start the wait.
	startTime := time.Now()

	// A helper function to simplify waiting locally.
	waitSynchronouslyFn := func() error {
		// Sleep
		time.Sleep(w.sleep)

		// Then, wait for child operations, if any
		if len(w.childOperations) > 0 {

			// Locally wait for each operation and contribute op status to wait group
			g := new(errgroup.Group)
			for _, childOperationName := range w.childOperations {
				g.Go(func() error {
					// Start loop to check if operation is done or timeout has passed
					for {
						operation, err := w.service.GetOperation(o.ctx, &longrunningpb.GetOperationRequest{Name: childOperationName})
						if err != nil {
							return err
						}
						// Operation is done, no futher action required.
						if operation.Done {
							return nil
						}

						// Check for timeouts.
						timePassed := time.Since(startTime)
						if timePassed.Seconds() > w.timeout.Seconds() {
							return ErrWaitDeadlineExceeded{
								message: fmt.Sprintf("operation (%s) exceeded timeout deadline of %0.0f seconds",
									childOperationName, w.timeout.Seconds()),
							}
						}
						// incur wait duration between polling
						time.Sleep(w.pollFrequency)
					}
				})
			}
			groupErr := g.Wait()
			if groupErr != nil {
				return groupErr
			}
		}
		return nil
	}

	// Async is not enabled, wait Synchronously
	if !w.asyncEnabled {
		err := waitSynchronouslyFn()
		if err != nil {
			return err
		}
	} else {
		// Wait asynchronously
		// Here we have two scenarios:
		//  - Production: Using Google Cloud Workflows to wait
		//  - Testing Locally: Using a callback function to call the relevant method again.

		// First, save the State and Resumepoint to Spanner before handing over.
		buffer := bytes.Buffer{}
		enc := gob.NewEncoder(&buffer)
		if err := enc.Encode(o.state); err != nil {
			return err
		}

		// Write the data to the database.
		_, err := o.client.spanner.Apply(o.ctx, []*spanner.Mutation{
			spanner.Update(
				o.client.spannerTable,
				[]string{"key", StateColumnName, ResumePointColumnName},
				[]any{o.name, buffer.Bytes(), w.resumePoint}),
		})
		if err != nil {
			return err
		}

		// Always wait locally when in dev mode, we'll use some seriously cool recursion magic 😎.
		if o.devMode {
			// First we'll wait asynchronously
			err := waitSynchronouslyFn()
			if err != nil {
				return err
			}
			// And then run the callback function to 'simulate' a resumable operation
			// We'll first add the operation id to the context
			operationId := strings.Split(o.name, "/")[1]
			ctx := context.WithValue(o.ctx, OperationIdHeaderKey, operationId)
			m, _ := metadata.FromIncomingContext(ctx)
			m.Delete("x-alis-forwarded-authorization")
			m.Delete("authorization")
			ctx = metadata.NewIncomingContext(ctx, m)
			go o.asyncCallbackFn(ctx)
		} else {
			// Hand over the wait task to Google Cloud Workflows.
			err := o.waitWithGoogleWorkflows(w)
			if err != nil {
				return err
			}
			return nil
		}
	}

	return nil
}

// waitWithGoogleWorkflows triggers asynchronous waiting in a workflow.
func (o *Operation[T]) waitWithGoogleWorkflows(cfg *WaitConfig) error {
	// Prepare the Google Cloud Workflow arguments
	type Args struct {
		OperationId            string   `json:"operationId"`
		InitialWaitDuration    int64    `json:"initialWaitDuration"`
		ChildOperations        []string `json:"childOperations"`
		PollFrequency          int64    `json:"pollFrequency"`
		PollEndpoint           string   `json:"pollEndpoint"`
		PollEndpointAudience   string   `json:"pollEndpointAudience"`
		ResumeEndpoint         string   `json:"resumeEndpoint"`
		ResumeEndpointAudience string   `json:"resumeEndpointAudience"`
		Timeout                int64    `json:"timeout"`
	}

	operationId := strings.Split(o.name, "/")[1]
	resumeEndpoint := o.client.resumeHost + o.resumeMethod
	args := Args{
		OperationId:            operationId,
		InitialWaitDuration:    int64(cfg.sleep.Seconds()),
		ChildOperations:        cfg.childOperations,
		PollFrequency:          int64(cfg.pollFrequency.Seconds()),
		PollEndpoint:           cfg.asyncChildGetOperationEndpoint,
		PollEndpointAudience:   "",
		ResumeEndpoint:         resumeEndpoint,
		ResumeEndpointAudience: "",
		Timeout:                int64(cfg.timeout.Seconds()),
	}

	// From the Resume Endpoint, get the Audience
	resumeUrl, err := url.Parse(resumeEndpoint)
	if err != nil {
		return fmt.Errorf("could not resolve resume url, invalid resume endpoint (%s): %w", resumeEndpoint, err)
	}
	args.ResumeEndpointAudience = "https://" + resumeUrl.Host

	// If there are any Child operations, configure the relevant arguments required by the polling mechanism used by Google Cloud Workflows.
	if len(cfg.childOperations) > 0 {
		// From the Poll Endpoint, get the Audience
		pollUrl, err := url.Parse(cfg.asyncChildGetOperationEndpoint)
		if err != nil {
			return fmt.Errorf("could not resolve poll url, invalid poll endpoint (%s): %w", cfg.asyncChildGetOperationEndpoint, err)
		}
		args.PollEndpointAudience = "https://" + pollUrl.Host
		args.ChildOperations = cfg.childOperations
	}

	// The Workflow service requires the argument in JSON format.
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return err
	}

	// Hand over the task of waiting to a dedicated Google Cloud Workflow
	_, err = o.client.workflows.CreateExecution(o.ctx, &executionspb.CreateExecutionRequest{
		Parent: o.client.workflowName,
		Execution: &executionspb.Execution{
			Argument:     string(argsBytes),
			CallLogLevel: executionspb.Execution_LOG_ALL_CALLS,
		},
	})
	if err != nil {
		// TODO: handle error types from Google explicitly.
		return fmt.Errorf("launch google cloud workflows: %w", err)
	}

	return nil
}
