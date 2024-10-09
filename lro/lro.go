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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"

	"go.alis.build/lro/internal/validate"
)

// WaitMechanism defines the waiting mechanisms that can be used to incur waiting
type WaitMechanism int32

const (
	Automatic WaitMechanism = iota
	Workflow
	LocalSleep
)

// OperationIdHeaderKey is use to indicate the the LRO already exists, and does not need to be created
const OperationIdHeaderKey = "x-alis-operation-id"

// Operation is the object used to manage the relevant LROs activties.
type Operation struct {
	ctx       context.Context
	client    *Client
	id        string
	operation *longrunningpb.Operation
	state     any
}

type StateType any

type WaitConfig struct {
	// child operation waiting options
	childOperations []string
	pollFrequency   time.Duration
	pollEndpoint    string

	// constant duration wait option
	waitDuration time.Duration

	// self wait option
	selfWait bool

	// resume options
	resumeConfig *ResumeConfig

	// general options
	waitMechanism WaitMechanism
	timeout       time.Duration
}

type ResumeConfig struct {
	ResumeEndpoint      string
	LocalResumeCallback func(context.Context)
	State               any
}

// Option is a functional option for the NewOperation method.
type WaitOption func(*WaitConfig) error

func NewResumableOperation[T StateType](ctx context.Context, client *Client) (*Operation, *T, error) {
	var err error
	op := &Operation{
		ctx:    context.WithoutCancel(ctx),
		client: client,
	}

	// In order to handle the resumable LRO design pattern, we add the relevant headers to the outgoing context.
	// Determine whether the operation argument is carried in context key x-alis-operation-id
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if len(md.Get(OperationIdHeaderKey)) > 0 {
			// We found a special header x-alis-operation-id, it suggests that the LRO is an existing one.
			op.id = md.Get(OperationIdHeaderKey)[0]
		}
	}

	if op.id == "" {
		// new operation, no resuming
		err = op.create()
		if err != nil {
			return nil, nil, err
		}
		return op, new(T), err
	} else {
		// if an existing LRO has been provided, resume by fetching op and state

		// get a copy of the LRO identified by op.id
		lro, err := op.client.Get(op.ctx, "operations/"+op.id)
		if err != nil {
			return nil, nil, err
		}
		op.id = strings.Split(lro.GetName(), "/")[1]
		op.operation = lro

		// get state of the LRO identified by op.id
		state, err := loadState[T](ctx, "operations/"+op.id, client)
		if err != nil {
			return op, nil, err
		}
		op.state = state
		// TODO consider combining the above two operations into one i.e. loadOpAndState

		return op, state, err
	}
}

/*
NewOperation creates a new Operation object used to simplify the management of the underlying LRO.

Example:

	op, err := lro.NewOperation(ctx, lroClient)
*/
func NewOperation(ctx context.Context, client *Client) (op *Operation, err error) {
	op = &Operation{
		ctx:    context.WithoutCancel(ctx),
		client: client,
	}

	err = op.create()
	if err != nil {
		return nil, err
	}

	return op, err
}

func GetOperation(ctx context.Context, client *Client, operation string) (op *Operation, err error) {
	op = &Operation{
		ctx:    context.WithoutCancel(ctx),
		client: client,
	}

	// Get a copy of the current LRO
	lro, err := op.client.Get(op.ctx, operation)
	if err != nil {
		return nil, err
	}

	op.id = strings.Split(lro.GetName(), "/")[1]
	op.operation = lro
	return op, err
}

// WithResumeConfig enables resume functionality on completion of waiting
func WithResumeConfig(resumeConfig *ResumeConfig) WaitOption {
	return func(w *WaitConfig) error {
		if resumeConfig == nil {
			return fmt.Errorf("resume config cannot be nil")
		}
		w.resumeConfig = resumeConfig
		return nil
	}
}

// WithWaitDuration sets a constant duration for which to wait.
// This should exceed the timeout.
func WithWaitDuration(waitDuration time.Duration) WaitOption {
	return func(w *WaitConfig) error {
		if waitDuration == 0 {
			return fmt.Errorf("wait duration cannot be zero")
		}
		w.waitDuration = waitDuration
		return nil
	}
}

// WithTimeout sets a timeout on waiting.
// If wait duration is specified, it should exceed the timeout.
func WithTimeout(timeout time.Duration) WaitOption {
	return func(w *WaitConfig) error {
		if timeout == 0 {
			return fmt.Errorf("timeout cannot be zero")
		}
		w.timeout = timeout
		return nil
	}
}

// WithWaitMechanism sets the wait mechanism.
// This option should only be used if the user intends to override the wait mechanism that is inferred from your environment.
func WithWaitMechanism(waitMechanism WaitMechanism) WaitOption {
	return func(w *WaitConfig) error {
		w.waitMechanism = waitMechanism
		return nil
	}
}

// ForChildOperations waits until all child operations are done, or timeout is exceeded.
func ForChildOperations(childOperations []string, pollFrequency time.Duration, pollEndpoint string) WaitOption {
	return func(w *WaitConfig) error {
		if len(childOperations) == 0 {
			return fmt.Errorf("child operations cannot be empty")
		}
		if pollFrequency == 0 {
			pollFrequency = 7 * time.Second
		}
		w.childOperations = childOperations
		w.pollFrequency = pollFrequency
		w.pollEndpoint = pollEndpoint
		return nil
	}
}

// ForSelf waits until the operation identified by operation.id is done.
func ForSelf(selfWait bool) WaitOption {
	return func(w *WaitConfig) error {
		w.selfWait = selfWait
		return nil
	}
}

// Wait blocks until the specified option(s) resolve.
func (o *Operation) Wait(opts ...WaitOption) error {
	// Store wait options on an instance of wait config such that waiting can be done in parallel on one operation
	w := &WaitConfig{
		waitMechanism: Automatic,
	}

	// Apply all wait options configured by user
	for _, opt := range opts {
		err := opt(w)
		// fail on error in option configuration
		if err != nil {
			return err
		}
	}

	// Timeout validation
	if w.timeout != 0 {
		// timeout is specified
		if w.waitDuration != 0 && w.timeout < w.waitDuration {
			// wait duration is specfied and timeout is specified as less than wait duration
			return fmt.Errorf("the configured wait duration explicilty exceeds the configured timeout: %s > %s", w.timeout, w.waitDuration)
		}
	} else {
		// timeout is not specified and therefore set to default
		w.timeout = 7 * time.Minute
		if w.waitDuration != 0 && w.timeout <= w.waitDuration {
			// wait duration is specified and timeout default is less than wait duration
			return fmt.Errorf("timeout is unset and default timeout is less than configured wait duration: %s > %s", w.timeout, w.waitDuration)
		}
	}

	// If wait mechanism is set to automatic,  Determine environment to resolve waiting mechanism (cloud workflows | time.Sleep)
	// Otherwise use user-set option
	if w.waitMechanism == Automatic {
		if os.Getenv("K_SERVICE") == "" {
			w.waitMechanism = LocalSleep
		} else {
			w.waitMechanism = Workflow
		}
	}

	// Save state if resume config is specified with state
	if w.resumeConfig != nil && w.resumeConfig.State != nil {
		saveState(o.ctx, o.operation.GetName(), o.client, w.resumeConfig.State)
	}

	startTime := time.Now()
	switch w.waitMechanism {
	case Workflow:

		// Prepare the Google Cloud Workflow argument
		type Args struct {
			OperationId            string   `json:"operationId"`
			InitialWaitDuration    int64    `json:"initialWaitDuration"`
			Operations             []string `json:"operations"`
			PollFrequency          int64    `json:"pollFrequency"`
			PollEndpoint           string   `json:"pollEndpoint"`
			PollEndpointAudience   string   `json:"pollEndpointAudience"`
			ResumeEndpoint         string   `json:"resumeEndpoint"`
			ResumeEndpointAudience string   `json:"resumeEndpointAudience"`
			Timeout                int64    `json:"timeout"`
		}
		args := Args{
			OperationId: o.id,
		}
		// 2. Configure workflow to incur waiting

		// 2 (a) Configure workflow to incur constant wait durations
		if w.waitDuration != 0 {
			args.InitialWaitDuration = int64(w.waitDuration.Seconds())
		}

		// 2 (b) Determine whether to include self in 'child operations'
		if w.selfWait {
			w.childOperations = append(w.childOperations, "operations/"+o.id)
		}

		// 2 (c) Configure workflow to wait for child operations
		if len(w.childOperations) > 0 {
			if args.PollEndpoint == "" {
				return fmt.Errorf("poll endpoint cannot be empty in the case of workflow waiting")
			}
			args.Operations = w.childOperations
			args.PollFrequency = int64(w.pollFrequency.Seconds())
			args.PollEndpoint = w.pollEndpoint
			pollUrl, err := url.Parse(w.pollEndpoint)
			if err != nil || pollUrl == nil {
				return fmt.Errorf("could not resolve pollUrl, invalid polling endpoint (%s): %w", w.pollEndpoint, err)
			}
			// update args given valid pollEndpoint
			args.PollEndpointAudience = "https://" + pollUrl.Host

			// Configure workflow's timeout for child operations, accounting for initial wait duration incurred
			timeoutQuotaAfterWaitDuration := w.timeout - w.waitDuration
			args.Timeout = int64(timeoutQuotaAfterWaitDuration.Seconds())
		}

		// 2 (d) Configure workflow's resume endpoint
		if w.resumeConfig != nil {
			if w.resumeConfig.ResumeEndpoint == "" {
				return fmt.Errorf("resume endpoint is required in resume config when wait mechanism is workflow")
			}
			args.ResumeEndpoint = w.resumeConfig.ResumeEndpoint
			// Process resumeEndpoint
			resumeUrl, err := url.Parse(w.resumeConfig.ResumeEndpoint)
			if err != nil {
				return fmt.Errorf("could not resolve resumeUrl, invalid resume endpoint (%s): %w", w.resumeConfig.ResumeEndpoint, err)
			}
			args.ResumeEndpointAudience = "https://" + resumeUrl.Host
		}

		// The Workflow service requires the argument in JSON format.
		argsBytes, err := json.Marshal(args)
		if err != nil {
			return err
		}

		// Launch Google Cloud Workflows to wait...
		_, err = o.client.workflows.CreateExecution(o.ctx, &executionspb.CreateExecutionRequest{
			Parent: o.client.workflowsConfig.name,
			Execution: &executionspb.Execution{
				Argument:     string(argsBytes),
				CallLogLevel: executionspb.Execution_LOG_ALL_CALLS,
			},
		})
		if err != nil {
			return err
		}

		return nil

	case LocalSleep:
		// 2. Incur waiting
		// 2 (a) First, incur constant wait durations
		// time.Sleep(w.waitDuration)
		time.Sleep(w.waitDuration)

		// 2 (b) Determine whether to include self in 'child operations'
		if w.selfWait {
			w.childOperations = append(w.childOperations, "operations/"+o.id)
		}

		// 2 (c) Then, wait for child operations, if any
		if len(w.childOperations) > 0 {
			// Configure workflow's timeout for child operations, accounting for initial wait duration incurred
			timeoutQuotaAfterWaitDuration := w.timeout - w.waitDuration

			g := new(errgroup.Group)
			// Locally wait for each operation and contribute op status to wait group
			for _, op := range w.childOperations {
				g.Go(func() error {
					// Start loop to check if operation is done or timeout has passed
					for {
						operation, err := o.client.Get(o.ctx, op)
						if err != nil {
							return err
						}
						if operation.Done {
							return nil
						}

						timePassed := time.Since(startTime)
						if timePassed.Seconds() > timeoutQuotaAfterWaitDuration.Seconds() {
							return ErrWaitDeadlineExceeded{
								message: fmt.Sprintf("operation (%s) exceeded timeout deadline of %0.0f seconds",
									op, timeoutQuotaAfterWaitDuration.Seconds()),
							}
						}
						time.Sleep(w.pollFrequency)
					}
				})
			}
			groupErr := g.Wait()
			if groupErr != nil {
				return groupErr
			}
		}

		// 3. Process resume config, if any
		if w.resumeConfig != nil {
			if w.resumeConfig.LocalResumeCallback == nil {
				return fmt.Errorf("local resume callback is required in resume config when wait mechanism is local sleep")
			}
			// attach opId to ctx to pick up existing op on callback
			// the callback that is provided is assumed to instantiate an op via NewOperation, which will look for existing ops that much the id in OperationIdHeaderKey
			newCtx := metadata.NewIncomingContext(o.ctx, metadata.Pairs(OperationIdHeaderKey, o.id))
			// user-specified callback to enable resuming after local waiting
			w.resumeConfig.LocalResumeCallback(newCtx)
		}

		return nil
	default:
		return fmt.Errorf("unknown wait mechanism: %d", w.waitMechanism)
	}
}

// SaveState saves a current state with the LRO resource
func saveState(ctx context.Context, operation string, client *Client, state any) error {
	buffer := bytes.Buffer{}
	enc := gob.NewEncoder(&buffer)
	if err := enc.Encode(state); err != nil {
		return err
	}

	err := client.spanner.UpdateRow(ctx, client.spannerConfig.Table, map[string]interface{}{
		"key":   operation,
		"State": buffer.Bytes(),
	})
	if err != nil {
		return err
	}

	return nil
}

// // loadState loads the current state stored with the LRO resource
// func loadState[T StateType]() (*T, error) {
// 	var data T
// 	row, err := o.client.spanner.ReadRow(o.ctx, o.client.spannerConfig.Table, spanner.Key{"operations/" + o.id}, []string{StateColumnName}, nil)
// 	if err != nil {
// 		return &data, err
// 	}
// 	if row[StateColumnName] != nil {

// 		stateString, ok := row[StateColumnName].(string)
// 		if !ok {
// 			return &data, fmt.Errorf("state data is not string")
// 		}

// 		// Decode from base643
// 		decodedData, err := base64.StdEncoding.DecodeString(stateString)
// 		if err != nil {
// 			fmt.Println("Error decoding:", err)
// 			return &data, err
// 		}

// 		var buffer bytes.Buffer
// 		buffer.Write(decodedData)
// 		decoder := gob.NewDecoder(&buffer)

// 		err = decoder.Decode(&data)
// 		if err != nil {
// 			return &data, err
// 		}
// 	}

// 	return &data, nil
// }

// loadState loads the current state stored with the LRO resource
func loadState[T StateType](ctx context.Context, operation string, client *Client) (*T, error) {
	var data T
	row, err := client.spanner.ReadRow(ctx, client.spannerConfig.Table, spanner.Key{operation}, []string{StateColumnName}, nil)
	if err != nil {
		return &data, err
	}
	if row[StateColumnName] != nil {

		stateString, ok := row[StateColumnName].(string)
		if !ok {
			return &data, fmt.Errorf("state data is not string")
		}

		// Decode from base64
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

// Get returns a long-running operation
func (o *Operation) GetOperation() (*longrunningpb.Operation, error) {
	return o.client.Get(o.ctx, "operations/"+o.id)
}

// ReturnRPC is mostly used by gRPC methods required to return gRPC compliant error codes.
func (o *Operation) ReturnRPC() (*longrunningpb.Operation, error) {
	op, err := o.client.Get(o.ctx, "operations/"+o.id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "retrieve operation: %s", err)
	}
	return op, nil
}

// SetSuccessful updates an existing long-running operation's done field to true, sets the response and updates the
// metadata if provided.
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

// Error updates an existing long-running operation's done field to true.
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

// UpdateMetadata updates an existing long-running operation's metadata.  Metadata typically
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

// Name returns the Long-running Operation resource name in the format 'operations/*'
func (o *Operation) GetName() string {
	return o.operation.GetName()
}

// Delete deletes the LRO (including )
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

// TODO: consider a Get that is not a method on Operation but rathen accepts an id and returns an Operation

// Get returns a long-running operation
func (o *Operation) Get() (*longrunningpb.Operation, error) {
	if o.id == "" {
		return nil, fmt.Errorf("operation id is nil")
	}
	return o.client.Get(o.ctx, "operations/"+o.id)
}

// wait constant duration
// wait for child operations
// wait for self
// option to resume
// override waitMechanism, typically automatic ( local sleep | workflows )
