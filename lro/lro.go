package lro

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/google/uuid"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Operation is the object used to manage the relevant LROs activties.
type Operation struct {
	ctx        context.Context
	client     *Client
	id         string
	operation  *longrunningpb.Operation
	waitConfig *WaitConfig
}

type WaitConfig struct {
	childOperations []*longrunningpb.Operation
	resumeConfig    *ResumeConfig[State]
	waitDuration    *time.Duration
	selfWait        bool
	devMode         bool
}

type ResumeConfig[T State] struct {
	resumeEndpoint      string
	localResumeCallback func(context.Context)
	state               *T
	devMode             bool
}

// Option is a functional option for the NewOperation method.
type Option func(*Operation)

// WithResumeConfig enables resume functionality on completion of waiting
func WithResumeConfig[T State](resumeConfig *ResumeConfig[State]) Option {
	return func(op *Operation) {
		op.waitConfig.resumeConfig = resumeConfig
	}
}

// WithResumeConfig enables resume functionality on completion of waiting
func WithWaitDuration(waitDuration *time.Duration) Option {
	return func(op *Operation) {
		op.waitConfig.waitDuration = waitDuration
	}
}

// ForChildOperations allows waiting until all child operations are done, or timeout is reached
func ForChildOperations(childOperations []*longrunningpb.Operation) Option {
	return func(op *Operation) {
		op.waitConfig.childOperations = childOperations
	}
}

// ForChildOperations allows waiting until all child operations are done, or timeout is reached
func ForSelf(selfWait bool) Option {
	return func(op *Operation) {
		op.waitConfig.selfWait = selfWait
	}
}

// Wait blocks until the specified option(s) resolve.
func (o *Operation) Wait(opts ...Option) error {
	for _, opt := range opts {
		opt(o)
	}

	return nil
}

/*
NewOperation creates a new Operation object used to simplify the management of the underlying LRO.

Example:

	op, err := lro.NewOperation(ctx, lroClient)
*/
func NewOperation(ctx context.Context, client *Client, operation string) (op *Operation, err error) {
	op = &Operation{
		ctx:    context.WithoutCancel(ctx),
		client: client,
	}

	// If an existing LRO has been provided, simply retrieve one
	if operation != "" {
		// Get a copy of the current LRO
		lro, err := op.client.Get(op.ctx, operation)
		if err != nil {
			return nil, err
		}
		op.id = strings.Split(lro.GetName(), "/")[1]
		op.operation = lro
	} else {
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
