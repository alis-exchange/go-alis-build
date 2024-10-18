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

// AsyncConfig is used when the particular op needs to wait asynchronously.
type AsyncConfig struct {
	// Host at which the workflow needs to be resumed at.
	// Example:
	Host string
	// Method to resume
	Method string

	// ResumeEndpoint is used by Google Cloud Workflows to resume the partciular method.
	ResumeEndpoint string
	// PollEndpoint specifies the enpoint Google Cloud Workflows should use to poll the underlygin Long-running Options.
	PollEndpoint string
	// LocalResumeCallback is used for local testing
	LocalResumeCallbackFn func(context.Context) error
}

// Operation is an object to to manage the lifecycle of a Google Longrunning-Operation.
type Operation[T any] struct {
	ctx         context.Context
	client      *Client
	name        string // The Operation resource name
	resumePoint string
	asyncConfig *AsyncConfig
	state       *T
}

type OperationOptions struct {
	host              string
	existingOperation string
	resumeMethod      string
}

// ClientOption is a functional option for the NewOperation method.
type OperationOption func(*OperationOptions)

// WithExistingOperation allows one to instantiate a new Operation object from an
// existing LRO. If this is provided the underlygin NewOperation method will
// not create a new Operation resource or infer it from the ctx.
func WithExistingOperation(name string) OperationOption {
	return func(opts *OperationOptions) {
		opts.existingOperation = name
	}
}

// WithResumeMethod sets the fully qualified method name to be resumed
// In most scenarios this will not be required since the method name will
// be automatically made avaialable by the grpc server exposed by the grpc.Method()
// Example: "/myorg.co.jobs.v1.JobsService/GenerateClientReports"
func WithResumeMethod(name string) OperationOption {
	return func(opts *OperationOptions) {
		opts.resumeMethod = name
	}
}

/*
NewOperation creates a new Operation object used to simplify the management of the underlying LRO.

Example:

	type MyState struct{}
	op, _ := lro.NewOperation[MyState](ctx, client)
*/
func NewOperation[T any](ctx context.Context, client *Client, opts ...OperationOption) (*Operation[T], error) {
	var err error
	// Configure the defualt options
	options := &OperationOptions{
		host:              "",
		existingOperation: "",
		resumeMethod:      "",
	}
	// Add the user provided overrides.
	for _, opt := range opts {
		opt(options)
	}

	// Construct an Operation object
	operation := &Operation[T]{
		ctx:         context.WithoutCancel(ctx),
		client:      client,
		name:        "",
		resumePoint: "",
		asyncConfig: &AsyncConfig{
			Host:                  "",
			Method:                "",
			ResumeEndpoint:        "",
			PollEndpoint:          "",
			LocalResumeCallbackFn: nil,
		},
		state: new(T),
	}

	// Set the Host details, if provided via options.
	if options.host != "" {
		operation.asyncConfig.Host = options.host
	} else {
		// Alternatively, default to the host set when the Client was instantiated.
		operation.asyncConfig.Host = client.workflowsResumeHost
	}

	// Set the method if the user specified it, otherwise ry to get from the context.
	if options.resumeMethod != "" {
		operation.asyncConfig.Method = options.resumeMethod
	} else {
		// Try to determine the method from the ctx
		methodName, ok := grpc.Method(operation.ctx)
		if ok {
			operation.asyncConfig.Method = methodName
		}
	}

	// If the user specified an existing Operation, use this, otherwise infer from ctx.
	if options.existingOperation != "" {
		operation.name = options.existingOperation
	} else {
		// In order to handle the resumable LRO design pattern, we add the relevant headers to the outgoing context.
		// Determine whether the operation argument is carried in context key x-alis-operation-id
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			if len(md.Get(OperationIdHeaderKey)) > 0 {
				// We found a special header x-alis-operation-id, it suggests that the LRO is an existing one.
				operation.name = "operations/" + md.Get(OperationIdHeaderKey)[0]
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

		// write operation and parent to respective spanner columns
		row := map[string]interface{}{"Operation": op}
		err = operation.client.spanner.InsertRow(operation.ctx, operation.client.spannerTable, row)
		if err != nil {
			return nil, err
		}
	} else {
		// The operation exists, get the details from the Spanner database.
		// No need to actually retrieve the Operation data from the database, only need the State and ResumePoint details, if available
		row, err := operation.client.spanner.ReadRow(operation.ctx, operation.client.spannerTable,
			spanner.Key{operation.name}, []string{StateColumnName, ResumePointColumnName}, nil)
		if err != nil {
			return nil, fmt.Errorf("read operation data from database: %w", err)
		}
		// Populate the State if available.
		if row[StateColumnName] != nil {
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

			err = decoder.Decode(&operation.state)
			if err != nil {
				return nil, fmt.Errorf("decode state")
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

	//  write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = o.client.spanner.UpsertRow(o.ctx, o.client.spannerTable, row)
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

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = o.client.spanner.UpsertRow(o.ctx, o.client.spannerTable, row)
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

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{"Operation": op}
	err = o.client.spanner.UpsertRow(o.ctx, o.client.spannerTable, row)
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
	err = o.client.spanner.DeleteRow(o.ctx, o.client.spannerTable, spanner.Key{o.name})
	if err != nil {
		return fmt.Errorf("delete operation (%s): %w", o.name, err)
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
	asyncCallbackFn                func(context.Context, *proto.Message) error
	asyncChildGetOperationEndpoint string // The API endpoint which exposes a GetOperation method
	asyncResumeEndpoint            string // The API endpoint which exposes a GetOperation method

	// Developer mode attributes
	devModeEnabled bool
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
func WithPollFrequency(timeout time.Duration) WaitOption {
	return func(w *WaitConfig) error {
		w.timeout = timeout
		return nil
	}
}

// WithChildOperations specifies operations for which to wait.
// Format: ["operations/123", "operations/456", "operations/789"]
func WithChildOperations(operations []string) WaitOption {
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

// WithCallbackFn is used for local development
func WithCallbackFn(fn func(context.Context, *proto.Message) error) WaitOption {
	return func(w *WaitConfig) error {
		w.asyncCallbackFn = fn
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

// WithResumeEndpoint sets the Endpoint the Google Cloud Workflow needs to hit at the end of its process to resume the operation.
func WithResumeEndpoint(endpoint string) WaitOption {
	return func(w *WaitConfig) error {
		w.asyncResumeEndpoint = endpoint
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
		service:                        nil,
		asyncEnabled:                   false,
		resumePoint:                    "",
		asyncCallbackFn:                nil,
		asyncChildGetOperationEndpoint: "",
		asyncResumeEndpoint:            "",
		devModeEnabled:                 true,
	}

	// We'll deactivate development mode if the service is running on Cloud Run
	if os.Getenv("K_SERVICE") != "" {
		w.devModeEnabled = false
	}

	// Set the default host, inline with google.longrunning package.
	w.asyncChildGetOperationEndpoint = o.asyncConfig.Host + "/google.longrunning.Operations/GetOperation"

	// From the parent Operation get the ResumeEndpoint
	w.asyncResumeEndpoint = o.asyncConfig.ResumeEndpoint

	// Now override any values explicity configured with the WaitOptions.
	for _, opt := range opts {
		err := opt(w)
		// fail on error in option configuration
		if err != nil {
			return err
		}
	}

	// If the service is not specified, use the default operations service configured at the client level.
	if w.service == nil {
		w.service = o.client
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

		if w.devModeEnabled {
			// First we'll wait asynchronously
			err := waitSynchronouslyFn()
			if err != nil {
				return err
			}
			// And then run the callback function to 'simulate' a resumable operation
			// TODO: make hit to callback function
			return w.asyncCallbackFn(o.ctx, nil)

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
	args := Args{
		OperationId:            operationId,
		InitialWaitDuration:    int64(cfg.sleep.Seconds()),
		ChildOperations:        cfg.childOperations,
		PollFrequency:          int64(cfg.pollFrequency.Seconds()),
		PollEndpoint:           cfg.asyncChildGetOperationEndpoint,
		PollEndpointAudience:   "",
		ResumeEndpoint:         cfg.asyncResumeEndpoint,
		ResumeEndpointAudience: "",
		Timeout:                int64(cfg.timeout.Seconds()),
	}

	// From the Resume Endpoint, get the Audience
	resumeUrl, err := url.Parse(cfg.asyncResumeEndpoint)
	if err != nil {
		return fmt.Errorf("could not resolve resume url, invalid resume endpoint (%s): %w", cfg.asyncResumeEndpoint, err)
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

	// Launch Google Cloud Workflows to wait...
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
