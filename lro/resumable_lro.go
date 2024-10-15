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
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/spanner"
	"cloud.google.com/go/workflows/executions/apiv1/executionspb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// ResumbaleOperation wraps an Operation, facilitating resumability of a method with the Operation.
type ResumableOperation struct {
	op         *Operation
	checkpoint *string
	// checkpointIndex       int
	// checkpointProgression []string
	resumeConfig *ResumeConfig
}

// ResumableOption is a functional option for a ResumableOperation.
type ResumableOption func(*ResumableOperation) error

// WithFirstCheckpoint sets the first checkpoint to execute in the resumable method.
func WithFirstCheckpoint(checkpoint string) ResumableOption {
	return func(r *ResumableOperation) error {
		r.checkpoint = &checkpoint
		return nil
	}
}

// WithResumeConfig sets config options for resume behaviour, including: resumeEndpoint, pollEndpoint (both of which can be inferred if client.workflowsConfig.Host is populated)
func WithResumeConfig(resumeConfig *ResumeConfig) ResumableOption {
	return func(r *ResumableOperation) error {
		if resumeConfig == nil {
			return fmt.Errorf("resume config cannot be nil")
		}
		r.resumeConfig = resumeConfig
		return nil
	}
}

// // WithCheckpointProgression sets the a progression of checkpoints to cycle through in the method, with waiting inbetween checkpoints
// func WithCheckpointProgression(progression []string) ResumableOption {
// 	return func(r *ResumableOperation) error {
// 		if len(progression) == 0 {
// 			return fmt.Errorf("checkpoint progressions cannot be empty")
// 		}
// 		r.checkpointProgression = progression
// 		return nil
// 	}
// }

// WithExistingOperation allows one to instantiate a new ResumableOperation object from an
// existing LRO.
func FromExistingOperation(operation string) ResumableOption {
	return func(r *ResumableOperation) error {
		if operation == "" {
			return fmt.Errorf("operation cannot be empty")
		}
		if !strings.HasPrefix(operation, "operations/") {
			return fmt.Errorf("invalid operation name")
		}
		operationId := strings.TrimPrefix(operation, "operations/")
		r.op.id = operationId
		return nil
	}
}

// withLocalResumableCallback is an internal option that is used to mock WaitAsync with WaitSync on local testing
func withLocalResumableCallback(localResumableCallback func(context.Context) error) WaitOption {
	return func(w *WaitConfig) error {
		w.callback = localResumableCallback
		return nil
	}
}

// // WithWaitMechanism sets the wait mechanism.
// // This option should only be used if the user intends to override the wait mechanism that is inferred from your environment.
// func WithWaitMechanism(waitMechanism WaitMechanism) WaitOption {
// 	return func(w *WaitConfig) error {
// 		w.waitMechanism = waitMechanism
// 		return nil
// 	}
// }

type StateType any

type ResumeConfig struct {
	ResumeEndpoint      string
	PollEndpoint        string
	LocalResumeCallback func(context.Context) error
}

// NewResumableOperation instantiates a new ResumableOperation object, optionally resuming from a pre-existing LRO.
func NewResumableOperation[T StateType](ctx context.Context, client *Client, opts ...ResumableOption) (*ResumableOperation, *T, error) {
	r := &ResumableOperation{
		op: &Operation{
			ctx:    context.WithoutCancel(ctx),
			client: client,
		},
	}

	// Apply all wait options configured by user
	for _, opt := range opts {
		err := opt(r)
		// fail on error in option configuration
		if err != nil {
			return nil, nil, err
		}
	}

	// In order to handle the resumable LRO design pattern, we add the relevant headers to the outgoing context.
	// Determine whether the operation argument is carried in context key x-alis-operation-id
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if len(md.Get(OperationIdHeaderKey)) > 0 {
			// We found a special header x-alis-operation-id, it suggests that the LRO is an existing one.
			r.op.id = md.Get(OperationIdHeaderKey)[0]
		}
	}

	if r.op.id == "" {
		// new operation, no resuming
		op, err := NewOperation(ctx, client)
		if err != nil {
			return nil, nil, err
		}
		r.op = op
		// r.checkpointIndex = 0
		// if r.checkpoint == nil && len(r.checkpointProgression) > 0 {
		// 	// don't save checkpoint progression if checkpoint's are being manually set
		// 	err = r.saveCheckpointProgression(r.checkpointProgression)
		// 	r.checkpoint = &r.checkpointProgression[0]
		// }
		return r, new(T), err
	} else {
		// if an existing LRO has been provided, resume by fetching rop and state

		// get a copy of the LRO identified by rop.id
		lro, err := r.op.client.Get(r.op.ctx, "operations/"+r.op.id)
		if err != nil {
			return nil, nil, err
		}
		r.op.id = strings.Split(lro.GetName(), "/")[1]
		r.op.operation = lro

		// get state of the LRO identified by r.op.id
		state, err := loadState[T](ctx, "operations/"+r.op.id, client)
		if err != nil {
			return nil, nil, err
		}
		// r.op.state = state

		r.checkpoint = nil

		checkpoint, _, _, err := r.loadCheckpointColumns()
		if err != nil {
			return nil, nil, err
		}

		// use checkpoint if explicitly set
		if checkpoint != "" {
			r.checkpoint = &checkpoint
			return r, state, err
		}

		// // use progression if no checkpoint is set
		// for i, ch := range checkpointProgression {
		// 	if i == checkpointIndex {
		// 		r.checkpoint = &ch
		// 		r.checkpointIndex = i
		// 	}
		// }

		// TODO consider combining the above two operations into one i.e. loadOpAndState

		return r, state, err
	}
}

// NextCheckpoint sets the checkpoint to resume at, overwriting any previously set checkpoint.
func (r *ResumableOperation) SetNextCheckpoint(checkpoint string) {
	r.checkpoint = &checkpoint
	r.saveCheckpoint(*r.checkpoint)
}

// GetCheckpoint retrieves the current checkpoint.
func (r *ResumableOperation) GetCheckpoint() string {
	if r.checkpoint == nil {
		return ""
	}
	return *r.checkpoint
}

// GetName returns the name of the underlying operation.
func (r *ResumableOperation) GetName() string {
	return r.op.GetName()
}

// GetOperation retrieves the underlying longrunningpb.Operation.
func (r *ResumableOperation) GetOperation() (*longrunningpb.Operation, error) {
	return r.op.GetOperation()
}

// ReturnRPC returns the underlying longrunningpb.Operation for further processing or sending to other APIs.
func (r *ResumableOperation) ReturnRPC() (*longrunningpb.Operation, error) {
	return r.op.ReturnRPC()
}

// Done marks the operation as done with a success response.
func (r *ResumableOperation) Done(response proto.Message) error {
	return r.op.Done(response)
}

// Error marks the operation as done with an error.
func (r *ResumableOperation) Error(error error) error {
	return r.op.Error(error)
}

// SetMetadata sets the metadata for the operation.
func (r *ResumableOperation) SetMetadata(metadata proto.Message) (*longrunningpb.Operation, error) {
	return r.op.SetMetadata(metadata)
}

// Get retrieves the current operation status.
func (r *ResumableOperation) Get() (*longrunningpb.Operation, error) {
	return r.op.Get()
}

// Wait blocks until the specified option(s) resolve.
// Wait accepts options to determine waiting behaviour
func (r *ResumableOperation) Wait(opts ...WaitOption) error {
	w := &WaitConfig{
		waitMechanism: Automatic,
	}

	// Consider allowing override of mechanism
	// // Apply all wait options configured by user
	// for _, opt := range opts {
	// 	err := opt(w)
	// 	// fail on error in option configuration
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	// If wait mechanism is set to automatic, determine environment to resolve waiting mechanism (cloud workflows | time.Sleep)
	// Otherwise use user-set option
	if w.waitMechanism == Automatic {
		if os.Getenv("K_SERVICE") == "" {
			w.waitMechanism = LocalSleep
		} else {
			w.waitMechanism = Workflow
		}
	}

	// // facilitate resume on next checkpoint in the case that checkpoint progression is being used and not r.SetNextCheckpoint
	// if r.checkpoint == nil && len(r.checkpointProgression) > 0 {
	// 	r.incrementAndSaveCheckpointIndex(r.checkpointIndex)
	// }

	switch w.waitMechanism {
	case Workflow:
		err := r.WaitAsync(opts...)
		if err != nil {
			return err
		}
	case LocalSleep:
		// if user has supplied a callback use that
		// otherwise configure a callback that mimics resumable hit via workflow
		if r.resumeConfig.LocalResumeCallback != nil {
			opts = append(opts, withLocalResumableCallback(r.resumeConfig.LocalResumeCallback))
		} else {
			opts = append(opts, withLocalResumableCallback(
				func(ctx context.Context) error {
					// TODO: consider using go.alis.build/test to implement this callback
					m, ok := grpc.Method(ctx)
					if !ok {
						return fmt.Errorf("failed to extract grpc.Method")
					}

					// TODO fix this, require client connection
					err := grpc.Invoke(ctx, m, nil, nil, nil)
					if err != nil {
						return fmt.Errorf("failed to grpc.Invoke on grpc.Method")
					}
					return nil
				},
			))
		}
		// mock WaitAsync using WaitSync and callback
		err := r.op.WaitSync(opts...)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown wait mechanism: %d", w.waitMechanism)
	}
	return nil
}

// WaitAsync triggers asynchronous waiting in a workflow.
func (r *ResumableOperation) WaitAsync(opts ...WaitOption) error {
	w := &WaitConfig{}

	// Apply all wait options configured by user
	for _, opt := range opts {
		err := opt(w)
		// fail on error in option configuration
		if err != nil {
			return err
		}
	}
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
		OperationId: r.op.id,
	}
	// 2. Configure workflow to incur waiting

	// 2 (a) Configure workflow to incur constant wait durations
	if w.sleep != 0 {
		args.InitialWaitDuration = int64(w.sleep.Seconds())
	}

	// 2 (b) Configure workflow to wait for child operations
	if len(w.operations) > 0 {
		if r.resumeConfig.PollEndpoint == "" {
			if r.op.client.workflowsConfig.Host == "" {
				return fmt.Errorf("poll endpoint is required in ResumeConfig if host is not set (failed to infer resume endpoint from workflows config host field)")
			}
			args.PollEndpoint = r.op.client.workflowsConfig.Host + "/google.longrunning.Operations/GetOperation"
		} else {
			args.PollEndpoint = r.resumeConfig.PollEndpoint
		}

		args.Operations = w.operations
		args.PollFrequency = int64(w.pollFrequency.Seconds())
		pollUrl, err := url.Parse(args.PollEndpoint)
		if err != nil || pollUrl == nil {
			return fmt.Errorf("could not resolve pollUrl, invalid polling endpoint (%s): %w", r.resumeConfig.PollEndpoint, err)
		}
		// update args given valid pollEndpoint
		args.PollEndpointAudience = "https://" + pollUrl.Host

		// timeout is not specified  therefore set to default
		if w.timeout == 0 {
			w.timeout = 7 * time.Minute
		}

		// Configure workflow's timeout for child operations, accounting for initial wait duration incurred
		args.Timeout = int64(w.timeout.Seconds())
	} else {
		args.Timeout = 0
		args.PollEndpoint = ""
		args.PollEndpointAudience = ""
		args.PollFrequency = 0
		args.Operations = []string{}
	}

	// 2 (c) Configure workflow's resume endpoint
	if r.resumeConfig.ResumeEndpoint == "" {
		if r.op.client.workflowsConfig.Host == "" {
			return fmt.Errorf("resume endpoint is required in ResumeConfig given the wait mechanism is workflow (failed to infer resume endpoint from workflows config host field)")
		}
		methodName, ok := grpc.Method(r.op.ctx)
		if !ok {
			return fmt.Errorf("resume endpoint is required in ResumeConfig given the wait mechanism is workflow (failed to infer resume endpoint from grpc.Method())")
		}
		r.resumeConfig.ResumeEndpoint = r.op.client.workflowsConfig.Host + methodName
	}
	args.ResumeEndpoint = r.resumeConfig.ResumeEndpoint
	resumeUrl, err := url.Parse(r.resumeConfig.ResumeEndpoint)
	if err != nil {
		return fmt.Errorf("could not resolve resumeUrl, invalid resume endpoint (%s): %w", r.resumeConfig.ResumeEndpoint, err)
	}
	args.ResumeEndpointAudience = "https://" + resumeUrl.Host

	// The Workflow service requires the argument in JSON format.
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return err
	}

	// Launch Google Cloud Workflows to wait...
	_, err = r.op.client.workflows.CreateExecution(r.op.ctx, &executionspb.CreateExecutionRequest{
		Parent: r.op.client.workflowsConfig.name,
		Execution: &executionspb.Execution{
			Argument:     string(argsBytes),
			CallLogLevel: executionspb.Execution_LOG_ALL_CALLS,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// SaveState saves the current state with the LRO resource
func (r *ResumableOperation) SaveState(state any) error {
	buffer := bytes.Buffer{}
	enc := gob.NewEncoder(&buffer)
	if err := enc.Encode(state); err != nil {
		return err
	}

	err := r.op.client.spanner.UpdateRow(r.op.ctx, r.op.client.spannerConfig.Table, map[string]interface{}{
		"key":           r.op.operation.GetName(),
		StateColumnName: buffer.Bytes(),
	})
	if err != nil {
		return err
	}

	return nil
}

// // SaveCheckpointProgression saves the checkpoint progression alongside an LRO resource
// func (r *ResumableOperation) saveCheckpointProgression(checkpointProgression []string) error {
// 	buffer := bytes.Buffer{}
// 	enc := gob.NewEncoder(&buffer)

// 	if err := enc.Encode(checkpointProgression); err != nil {
// 		return err
// 	}

// 	err := r.op.client.spanner.UpdateRow(r.op.ctx, r.op.client.spannerConfig.Table, map[string]interface{}{
// 		"key":                           r.op.operation.GetName(),
// 		CheckpointIndexColumnName:       0,
// 		CheckpointProgressionColumnName: checkpointProgression,
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// // SaveCheckpointIndex saves the checkpoint index alongside an LRO resource
// func (r *ResumableOperation) incrementAndSaveCheckpointIndex(checkpointIndex int) error {
// 	incrementedIndex := checkpointIndex + 1
// 	buffer := bytes.Buffer{}
// 	enc := gob.NewEncoder(&buffer)

// 	if err := enc.Encode(incrementedIndex); err != nil {
// 		return err
// 	}

// 	err := r.op.client.spanner.UpdateRow(r.op.ctx, r.op.client.spannerConfig.Table, map[string]interface{}{
// 		"key":                     r.op.operation.GetName(),
// 		CheckpointIndexColumnName: incrementedIndex,
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// SaveCheckpoint saves the checkpoint alongside an LRO resource
func (r *ResumableOperation) saveCheckpoint(checkpoint string) error {
	buffer := bytes.Buffer{}
	enc := gob.NewEncoder(&buffer)

	if err := enc.Encode(checkpoint); err != nil {
		return err
	}

	err := r.op.client.spanner.UpdateRow(r.op.ctx, r.op.client.spannerConfig.Table, map[string]interface{}{
		"key":                r.op.operation.GetName(),
		CheckpointColumnName: checkpoint,
	})
	if err != nil {
		return err
	}

	return nil
}

// loadCheckpointColumns loads the checkpoint, checkpoint index and checkpoint progression stored with the LRO resource
func (r *ResumableOperation) loadCheckpointColumns() (string, int, []string, error) {
	row, err := r.op.client.spanner.ReadRow(r.op.ctx, r.op.client.spannerConfig.Table, spanner.Key{r.op.operation.GetName()}, []string{CheckpointColumnName, CheckpointIndexColumnName, CheckpointProgressionColumnName}, nil)
	if err != nil {
		return "", 0, nil, err
	}

	if row[CheckpointColumnName] == nil {
		if row[CheckpointProgressionColumnName] == nil {
			return "", 0, nil, fmt.Errorf("checkpoint progression is nil")
		}
		checkpointProgression := []string{}
		for _, x := range row[CheckpointProgressionColumnName].([]interface{}) {
			if str, ok := x.(string); ok {
				checkpointProgression = append(checkpointProgression, str)
			} else {
				return "", 0, nil, fmt.Errorf("checkpoint progression item is not string")
			}
		}

		if row[CheckpointIndexColumnName] == nil {
			return "", 0, nil, fmt.Errorf("checkpoint index is nil")
		}
		checkpointIndex, ok := row[CheckpointIndexColumnName].(string)
		if !ok {
			return "", 0, nil, fmt.Errorf("checkpoint index data is not int64")
		}
		intVal, err := strconv.ParseInt(checkpointIndex, 10, 64)
		if err != nil {
			return "", 0, nil, fmt.Errorf("strconv.ParseInt: convert checkpoint index to integer")
		}
		return "", int(intVal), checkpointProgression, nil
	} else {
		checkpoint, ok := row[CheckpointColumnName].(string)
		if !ok {
			return "", 0, nil, fmt.Errorf("checkpoint data is not string")
		}
		return checkpoint, 0, nil, nil
	}
}

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
