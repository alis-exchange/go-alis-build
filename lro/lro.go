package lro

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/spanner"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"

	"go.alis.build/lro/internal/validate"
)

// OperationIdHeaderKey is use to indicate the the LRO already exists, and does not need to be created
const OperationIdHeaderKey = "x-alis-operation-id"

// Operation is the object used to manage activities on a longrunningpb.Operation
type Operation struct {
	ctx       context.Context
	client    *Client
	id        string
	operation *longrunningpb.Operation
}

// WaitConfig is used to store the waiting configurations specified as functional WaitOption(s) in Wait()
type WaitConfig struct {
	// child operation waiting options
	operations    []string
	pollFrequency time.Duration
	pollClient    *Client

	// constant duration wait option
	sleep time.Duration

	timeout time.Duration

	callback func(context.Context) error

	waitMechanism WaitMechanism
}

// WaitOption is a functional option for WaitConfig.
type WaitOption func(*WaitConfig) error

// NewOperationOption is a functional option for Operation.
type NewOperationOption func(*Operation) error

// WaitMechanism defines the waiting mechanisms that can be used to incur waiting.
type WaitMechanism int32

const (
	// The waiting mechanism is determined by the environment
	Automatic WaitMechanism = iota
	// The waiting mechanism is a Google Cloud Workflow
	Workflow
	// The waiting mechanism is time.Sleep in the thread that calls Wait
	LocalSleep
)

// WithExistingOperation allows one to instantiate a new Operation object from an
// existing LRO, using the name of the LRO.
func WithExistingOperation(operation string) NewOperationOption {
	return func(o *Operation) error {
		if operation == "" {
			return fmt.Errorf("operation cannot be empty")
		}
		if !strings.HasPrefix(operation, "operations/") {
			return fmt.Errorf("invalid operation name")
		}
		operationId := strings.TrimPrefix(operation, "operations/")
		o.id = operationId
		return nil
	}
}

/*
NewOperation creates a new Operation object used to simplify the management of the underlying LRO.

Example:

	op, err := lro.NewOperation(ctx, lroClient)
*/
func NewOperation(ctx context.Context, client *Client, opts ...NewOperationOption) (op *Operation, err error) {
	op = &Operation{
		ctx:    context.WithoutCancel(ctx),
		client: client,
	}
	for _, opt := range opts {
		opt(op)
	}

	// operation id set with WithExistingOperation, therefore retrieve the lro and wrap in Operation struct
	if op.id != "" {
		// Get a copy of the current LRO
		lro, err := op.client.Get(op.ctx, "operations/"+op.id)
		if err != nil {
			return nil, err
		}
		op.id = strings.Split(lro.GetName(), "/")[1]
		op.operation = lro
	} else {
		// create a new lro
		err = op.create()
		if err != nil {
			return nil, err
		}
	}

	return op, err
}

// create stores a new long-running operation in spanner, with done=false
func (o *Operation) create() error {
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

// WithSleep specified a constant duration for which to wait.
func WithSleep(sleep time.Duration) WaitOption {
	return func(w *WaitConfig) error {
		if sleep == 0 {
			return fmt.Errorf("sleep duration cannot be zero")
		}
		w.sleep = sleep
		return nil
	}
}

// ForOperations specifies operations for which to wait.
// Format: ["operations/1", "operations/2", "operations/3"]
// NOTE: Insert an Operations own name in operations in order to block until the operation is set to done.
// TODO: change this to WithOperations if preferred
func ForOperations(operations []string, pollFrequency time.Duration) WaitOption {
	return func(w *WaitConfig) error {
		if len(operations) == 0 {
			return fmt.Errorf("operations cannot be empty")
		}
		if pollFrequency == 0 {
			return fmt.Errorf("poll frequency cannot be zero")
		}
		// if pollClient == nil {
		// 	return fmt.Errorf("poll client cannot be nil")
		// }
		// w.pollClient = pollClient
		w.operations = operations
		w.pollFrequency = pollFrequency
		return nil
	}
}

// ForOperationsWithTimeout performs the same function as ForOperations except a timeout is applied as an upper limit on waiting.
// TODO consider accepting a map[string]*Client for operations where each operation client can be specfified along side it, if nil default to o.client.
func ForOperationsWithTimeout(operations []string, pollFrequency time.Duration, timeout time.Duration) WaitOption {
	return func(w *WaitConfig) error {
		if len(operations) == 0 {
			return fmt.Errorf("operations cannot be empty")
		}
		if pollFrequency == 0 {
			return fmt.Errorf("poll frequency cannot be zero")
		}
		if timeout == 0 {
			return fmt.Errorf("timeout cannot be zero")
		}
		// if pollClient == nil {
		// return fmt.Errorf("poll client cannot be nil")
		// }
		// w.pollClient = pollClient
		w.operations = operations
		w.pollFrequency = pollFrequency
		w.timeout = timeout
		return nil
	}
}

// WaitSync blocks until the specified option(s) resolve, and then continues execution.
// WaitSync accepts options to determine waiting behaviour.
func (o *Operation) WaitSync(opts ...WaitOption) error {
	// Store wait options on an instance of wait config such that waiting can be done in parallel on one operation
	w := &WaitConfig{}

	// Apply all wait options configured by user
	for _, opt := range opts {
		err := opt(w)
		// fail on error in option configuration
		if err != nil {
			return err
		}
	}

	// Client is not specified therefore set default as current lro client
	if w.pollClient == nil {
		w.pollClient = o.client
	}

	// Timeout is not specified therefore set default
	if w.timeout == 0 {
		w.timeout = 7 * time.Minute
	}

	startTime := time.Now()

	// 2. Incur waiting
	// 2 (a) First, incur constant wait durations
	time.Sleep(w.sleep)

	// 2 (b) Then, wait for child operations, if any
	if len(w.operations) > 0 {

		g := new(errgroup.Group)
		// Locally wait for each operation and contribute op status to wait group
		for _, op := range w.operations {
			g.Go(func() error {
				// Start loop to check if operation is done or timeout has passed
				for {
					operation, err := w.pollClient.Get(o.ctx, op)
					if err != nil {
						return err
					}
					if operation.Done {
						return nil
					}

					timePassed := time.Since(startTime)
					if timePassed.Seconds() > w.timeout.Seconds() {
						return ErrWaitDeadlineExceeded{
							message: fmt.Sprintf("operation (%s) exceeded timeout deadline of %0.0f seconds",
								op, w.timeout.Seconds()),
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

	// 2 (c) Process resume config, if any
	// This is here to facilitate the use of WaitSync to mock WaitAsync when testing
	// i.e., the callback replaces the function of a workflow hit on resumeEndpoint
	if w.callback != nil {
		// attach opId to ctx to pick up existing op on callback
		// the callback that is provided is assumed to instantiate an op via NewOperation, which will look for existing ops that match the id in OperationIdHeaderKey
		newCtx := metadata.NewIncomingContext(o.ctx, metadata.Pairs(OperationIdHeaderKey, o.id))
		// user-specified callback to enable resuming after local waiting
		err := w.callback(newCtx)
		if err != nil {
			return err
		}
	}

	return nil
}

// Get returns a long-running operation
func (o *Operation) GetOperation() (*longrunningpb.Operation, error) {
	return o.client.Get(o.ctx, "operations/"+o.id)
}

// ReturnRPC returns the operation, or a gRPC compliant error code.
func (o *Operation) ReturnRPC() (*longrunningpb.Operation, error) {
	op, err := o.client.Get(o.ctx, "operations/"+o.id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "retrieve operation: %s", err)
	}
	return op, nil
}

// Done updates the done field to true and sets the result to the response
func (o *Operation) Done(response proto.Message) error {
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
	err = o.client.spanner.UpsertRow(o.ctx, o.client.spannerConfig.Table, row)
	if err != nil {
		return err
	}

	return nil
}

// Error updates an existing long-running operation's done field to true and sets the error field
func (o *Operation) Error(error error) error {
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
	err = o.client.spanner.UpsertRow(o.ctx, o.client.spannerConfig.Table, row)
	if err != nil {
		return err
	}

	return nil
}

// SetMetadata updates an existing long-running operation's metadata.  Metadata typically
// contains progress information and common metadata such as create time.
func (o *Operation) SetMetadata(metadata proto.Message) (*longrunningpb.Operation, error) {
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
	err = o.client.spanner.UpsertRow(o.ctx, o.client.spannerConfig.Table, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// GetName returns the Long-running Operation resource name in the format 'operations/*'
func (o *Operation) GetName() string {
	return o.operation.GetName()
}

// Delete deletes the LRO, including auxiliary columns
func (o *Operation) Delete() (*emptypb.Empty, error) {
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

// Get returns a long-running operation
func (o *Operation) Get() (*longrunningpb.Operation, error) {
	if o.id == "" {
		return nil, fmt.Errorf("operation id is nil")
	}
	return o.client.Get(o.ctx, "operations/"+o.id)
}
