package lro

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"go.alis.build/alog"
	rpcStatus "google.golang.org/genproto/googleapis/rpc/status"
	proto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Client manages long-running operations for a specific neuron.
type Client struct {
	host        string
	db          *database
	mux         *http.ServeMux
	taskQueue   *queue
	muxPatterns *sync.Map
}

// Options configures a Client created with New.
type Options struct {
	host string
}

// Option configures a Client during New.
type Option func(*Options)

// WithHost overrides the default Cloud Run host used for resumable operation callbacks.
func WithHost(host string) Option {
	return func(opts *Options) {
		opts.host = host
	}
}

func defaultHost(neuron string) (string, error) {
	runHash := os.Getenv("ALIS_RUN_HASH")
	if runHash == "" {
		return "", fmt.Errorf("ALIS_RUN_HASH not set")
	}
	return fmt.Sprintf("https://%s-backend-%s.run.app", neuron, runHash), nil
}

// New constructs a new LRO client for the given neuron and HTTP mux.
func New(neuron string, mux *http.ServeMux, opts ...Option) (*Client, error) {
	if neuron == "" {
		return nil, fmt.Errorf("neuron is required")
	}
	if mux == nil {
		return nil, fmt.Errorf("mux is required")
	}

	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	host := options.host
	if host == "" {
		var err error
		host, err = defaultHost(neuron)
		if err != nil {
			return nil, err
		}
	}

	db, err := newDB(neuron)
	if err != nil {
		return nil, err
	}
	taskQueue, err := newQueue(neuron)
	if err != nil {
		return nil, err
	}

	return &Client{
		host:        host,
		db:          db,
		mux:         mux,
		taskQueue:   taskQueue,
		muxPatterns: &sync.Map{},
	}, nil
}

// Close closes the underlying Spanner and Cloud Tasks clients.
func (c *Client) Close() error {
	if c.db != nil && c.db.Client != nil {
		c.db.Client.Close()
	}
	if c.taskQueue != nil && c.taskQueue.client != nil {
		return c.taskQueue.client.Close()
	}
	return nil
}

// Operation is a long-running operation managed by a Client.
type Operation struct {
	row    *OperationRow
	Ctx    context.Context
	client *Client
}

// NewOperation creates a new operation row and stores its initial metadata.
func (c *Client) NewOperation(ctx context.Context, operationName string, md proto.Message) (*Operation, error) {
	method := fmt.Sprintf("%T", md)
	method = strings.TrimSuffix(method, "Metadata")
	parts := strings.Split(method, ".")
	method = parts[len(parts)-1]

	opRow := &OperationRow{
		Operation: &longrunningpb.Operation{
			Name: operationName,
		},
		Method: method,
	}
	op := &Operation{
		row:    opRow,
		Ctx:    ctx,
		client: c,
	}
	op.SetMetadata(md)

	if err := c.db.Insert(ctx, opRow); err != nil {
		return nil, fmt.Errorf("inserting operation row: %w", err)
	}
	return op, nil
}

// GetOperationPb retrieves the underlying protobuf operation by name.
func (c *Client) GetOperationPb(ctx context.Context, name string) (*longrunningpb.Operation, error) {
	opRow, err := c.db.Read(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("getting operation row: %w", err)
	}
	return opRow.Operation, nil
}

// GetOperation retrieves an operation wrapper by operation name.
func (c *Client) GetOperation(ctx context.Context, operationName string) (*Operation, error) {
	opRow, err := c.db.Read(ctx, operationName)
	if err != nil {
		return nil, err
	}
	return &Operation{
		row:    opRow,
		Ctx:    ctx,
		client: c,
	}, nil
}

// OperationPb returns the underlying protobuf operation.
func (o *Operation) OperationPb() *longrunningpb.Operation {
	return o.row.Operation
}

// ResumeViaTasks saves the operation and schedules the supplied handler to resume later via Cloud Tasks.
func (o *Operation) ResumeViaTasks(handler func(op *Operation), waitDuration time.Duration) error {
	o.Save()

	muxPattern := "/operations/" + o.row.Method
	url := o.client.host + muxPattern + "?operation=" + o.row.Operation.GetName()
	if _, loaded := o.client.muxPatterns.LoadOrStore(muxPattern, struct{}{}); !loaded {
		o.client.mux.HandleFunc("PUT "+muxPattern, func(w http.ResponseWriter, r *http.Request) {
			opName := r.URL.Query().Get("operation")
			if opName == "" {
				http.Error(w, "missing operation query param", http.StatusBadRequest)
				return
			}

			opRow, err := o.client.db.Read(r.Context(), opName)
			if err != nil {
				http.Error(w, fmt.Sprintf("reading operation row: %v", err), http.StatusInternalServerError)
				return
			}
			handler(&Operation{
				row:    opRow,
				Ctx:    r.Context(),
				client: o.client,
			})
			w.WriteHeader(http.StatusOK)
		})
	}

	if err := o.client.taskQueue.schedulePutRequest(o.Ctx, o.client.mux, url, time.Now().Add(waitDuration)); err != nil {
		o.Fail("scheduling cloudtask: %v", err)
		return fmt.Errorf("scheduling cloudtask: %w", err)
	}
	return nil
}

// MustUnmarshalMetadata unmarshals operation metadata into md and fatals on error.
func MustUnmarshalMetadata[MdT proto.Message](op *Operation, md MdT) MdT {
	if err := anypb.UnmarshalTo(op.row.Operation.GetMetadata(), md, proto.UnmarshalOptions{}); err != nil {
		alog.Fatalf(context.Background(), "unmarshalling metadata from any: %v", err)
	}
	return md
}

// UnmarshalMetadata unmarshals operation metadata into md.
func UnmarshalMetadata[MdT proto.Message](op *Operation, md MdT) (MdT, error) {
	if err := anypb.UnmarshalTo(op.row.Operation.GetMetadata(), md, proto.UnmarshalOptions{}); err != nil {
		return md, err
	}
	return md, nil
}

// SaveMetadata updates the operation metadata and persists the operation.
func (o *Operation) SaveMetadata(md proto.Message) {
	o.SetMetadata(md)
	o.Save()
}

// SetMetadata updates the operation metadata without persisting the operation.
func (o *Operation) SetMetadata(md proto.Message) {
	mdAny, err := anypb.New(md)
	if err != nil {
		alog.Fatalf(context.Background(), "marshalling metadata into any: %v", err)
	}
	o.row.Operation.Metadata = mdAny
}

// DecodePrivateState decodes the operation's private state into state.
func (o *Operation) DecodePrivateState(state any) {
	r := bytes.NewReader(o.row.State)
	dec := gob.NewDecoder(r)
	if err := dec.Decode(state); err != nil {
		alog.Fatalf(context.Background(), "gob decoding private state: %v", err)
	}
}

// SavePrivateState updates the private state and persists the operation.
func (o *Operation) SavePrivateState(state any) {
	o.SetPrivateState(state)
	o.Save()
}

// SetPrivateState updates the private state without persisting the operation.
func (o *Operation) SetPrivateState(state any) {
	w := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(w)
	if err := enc.Encode(state); err != nil {
		alog.Fatalf(context.Background(), "gob encoding state: %v", err)
	}
	o.row.State = w.Bytes()
}

// ResumePoint returns the operation's current resume point.
func (o *Operation) ResumePoint() string {
	return o.row.ResumePoint
}

// SaveResumePoint updates the resume point and persists the operation.
func (o *Operation) SaveResumePoint(resumePoint string) {
	o.SetResumePoint(resumePoint)
	o.Save()
}

// SetResumePoint updates the resume point without persisting the operation.
func (o *Operation) SetResumePoint(resumePoint string) {
	o.row.ResumePoint = resumePoint
}

// Save persists the current operation row.
func (o *Operation) Save() {
	if err := o.client.db.Update(o.Ctx, o.row); err != nil {
		alog.Fatalf(o.Ctx, "saving operation row: %v", err)
	}
}

// Complete marks the operation done with a successful response and persists it.
func (o *Operation) Complete(resp proto.Message) {
	if resp == nil {
		alog.Alertf(o.Ctx, "completing operation with nil response: %s", o.row.Operation.GetName())
		return
	}

	respAny, err := anypb.New(resp)
	if err != nil {
		alog.Alertf(o.Ctx, "marshalling response into any: %v", err)
		return
	}

	o.row.Operation.Done = true
	o.row.Operation.Result = &longrunningpb.Operation_Response{Response: respAny}
	o.Save()
}

// Fail marks the operation done with an error message and persists it.
func (o *Operation) Fail(reason string, args ...any) {
	o.row.Operation.Done = true
	o.row.Operation.Result = &longrunningpb.Operation_Error{
		Error: &rpcStatus.Status{
			Message: fmt.Sprintf(reason, args...),
		},
	}
	o.Save()
}
