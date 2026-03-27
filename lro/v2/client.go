package lro

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
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
	host              string
	db                *database
	mux               *http.ServeMux
	muxPrefix         string
	taskQueue         *queue
	muxPatterns       *sync.Map
	resumableHandlers *sync.Map
}

// Config configures a Client created with New.
type Config struct {
	Neuron string

	// Project is the owning project used to derive the operations table name.
	Project string

	SpannerProject  string
	SpannerInstance string
	SpannerDatabase string
	DatabaseRole    string

	CloudTasksProject        string
	CloudTasksLocation       string
	CloudTasksQueue          string
	CloudTasksServiceAccount string

	Host string
}

type options struct {
	host *string
}

// Option configures env-derived client construction.
type Option func(*options)

// WithHost overrides the callback host used by NewFromEnv.
func WithHost(host string) Option {
	return func(opts *options) {
		opts.host = &host
	}
}

// New constructs a new LRO client from explicit configuration.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	location, err := resolveCloudTasksLocation(cfg.CloudTasksLocation)
	if err != nil {
		return nil, err
	}
	cfg.CloudTasksLocation = location

	db, err := newDB(ctx, cfg)
	if err != nil {
		return nil, err
	}
	taskQueue, err := newQueue(ctx, cfg)
	if err != nil {
		if db != nil && db.Client != nil {
			db.Client.Close()
		}
		return nil, err
	}

	return &Client{
		host:              cfg.Host,
		db:                db,
		taskQueue:         taskQueue,
		muxPatterns:       &sync.Map{},
		resumableHandlers: &sync.Map{},
	}, nil
}

func validateConfig(cfg Config) error {
	if cfg.Neuron == "" {
		return fmt.Errorf("neuron is required")
	}
	if cfg.Project == "" {
		return fmt.Errorf("project is required")
	}
	if cfg.SpannerProject == "" {
		return fmt.Errorf("spanner project is required")
	}
	if cfg.SpannerInstance == "" {
		return fmt.Errorf("spanner instance is required")
	}
	if cfg.SpannerDatabase == "" {
		return fmt.Errorf("spanner database is required")
	}
	if cfg.CloudTasksProject == "" {
		return fmt.Errorf("cloud tasks project is required")
	}
	if cfg.CloudTasksLocation == "" {
		return fmt.Errorf("cloud tasks location is required")
	}
	if _, err := resolveCloudTasksLocation(cfg.CloudTasksLocation); err != nil {
		return err
	}
	if cfg.CloudTasksQueue == "" {
		return fmt.Errorf("cloud tasks queue is required")
	}
	if cfg.CloudTasksServiceAccount == "" {
		return fmt.Errorf("cloud tasks service account is required")
	}
	if cfg.Host == "" {
		return fmt.Errorf("host is required")
	}
	return nil
}

// RegisterHTTPHandlers registers the default operations callback route on mux.
func (c *Client) RegisterHTTPHandlers(mux *http.ServeMux) error {
	return c.RegisterHTTPHandlersAtPrefix(mux, "/operations/")
}

// RegisterHTTPHandlersAtPrefix registers resumable operation callbacks using the supplied path prefix.
func (c *Client) RegisterHTTPHandlersAtPrefix(mux *http.ServeMux, prefix string) error {
	if mux == nil {
		return fmt.Errorf("mux is required")
	}

	prefix = normalizePrefix(prefix)
	c.mux = mux
	c.muxPrefix = prefix

	var registerErr error
	c.resumableHandlers.Range(func(key, _ any) bool {
		handlerPath, _ := key.(string)
		if err := c.registerHTTPHandlerForPath(handlerPath); err != nil {
			registerErr = err
			return false
		}
		return true
	})
	if registerErr != nil {
		return registerErr
	}
	return nil
}

// ResumeHandler resumes a previously scheduled long-running operation.
type ResumeHandler func(*Operation)

// ResumableHandler associates a callback path with the handler that resumes that operation.
type ResumableHandler struct {
	Path    string
	Handler ResumeHandler
}

// AddResumableHandler associates a callback path with the handler that resumes that operation.
func (c *Client) AddResumableHandler(path string, handler ResumeHandler) error {
	if path == "" {
		return fmt.Errorf("handler path is required")
	}
	if handler == nil {
		return fmt.Errorf("handler is required")
	}
	if _, loaded := c.resumableHandlers.LoadOrStore(path, handler); loaded {
		return fmt.Errorf("resumable handler already registered for path %q", path)
	}
	if c.mux != nil {
		if err := c.registerHTTPHandlerForPath(path); err != nil {
			return err
		}
	}
	return nil
}

// AddResumableHandlers adds multiple resumable handlers to the client.
func (c *Client) AddResumableHandlers(handlers ...ResumableHandler) error {
	for _, resumableHandler := range handlers {
		if err := c.AddResumableHandler(resumableHandler.Path, resumableHandler.Handler); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) registerHTTPHandlerForPath(path string) error {
	if c.mux == nil {
		return fmt.Errorf("mux is required")
	}
	if _, ok := c.resumableHandlers.Load(path); !ok {
		return fmt.Errorf("no resumable handler registered for path %q", path)
	}

	muxPattern := c.muxPrefix + path
	if _, loaded := c.muxPatterns.LoadOrStore(muxPattern, struct{}{}); loaded {
		return nil
	}

	c.mux.HandleFunc("PUT "+muxPattern, func(w http.ResponseWriter, r *http.Request) {
		handlerValue, ok := c.resumableHandlers.Load(path)
		if !ok {
			http.Error(w, fmt.Sprintf("no resumable handler registered for path %q", path), http.StatusInternalServerError)
			return
		}
		handler, _ := handlerValue.(ResumeHandler)

		opName := r.URL.Query().Get("operation")
		if opName == "" {
			http.Error(w, "missing operation query param", http.StatusBadRequest)
			return
		}

		opRow, err := c.db.Read(r.Context(), opName)
		if err != nil {
			http.Error(w, fmt.Sprintf("reading operation row: %v", err), http.StatusInternalServerError)
			return
		}
		handler(&Operation{
			row:    opRow,
			Ctx:    r.Context(),
			client: c,
		})
		w.WriteHeader(http.StatusOK)
	})
	return nil
}

func normalizePrefix(prefix string) string {
	if prefix == "" {
		return "/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
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

// OperationsServer returns a google.longrunning.Operations server backed by the client.
func (c *Client) OperationsServer(opts ...OperationsServerOption) *OperationsServer {
	return NewOperationsServer(c, opts...)
}

// Operation is a long-running operation managed by a Client.
type Operation struct {
	row    *OperationRow
	Ctx    context.Context
	client *Client
}

// NewOperation creates a new operation row and stores its initial metadata.
func (c *Client) NewOperation(ctx context.Context, operationName string, md proto.Message) (*Operation, error) {
	opRow := &OperationRow{
		Operation: &longrunningpb.Operation{
			Name: operationName,
		},
	}
	op := &Operation{
		row:    opRow,
		Ctx:    ctx,
		client: c,
	}
	if err := op.SetMetadata(md); err != nil {
		return nil, err
	}

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

// ResumeViaTasks saves the operation and schedules the registered resume path to run later via Cloud Tasks.
func (o *Operation) ResumeViaTasks(path string, waitDuration time.Duration) error {
	if o.client.mux == nil {
		return fmt.Errorf("client HTTP handlers are not registered; call RegisterHTTPHandlers or RegisterHTTPHandlersAtPrefix first")
	}
	if path == "" {
		return fmt.Errorf("handler path is required")
	}
	if _, ok := o.client.resumableHandlers.Load(path); !ok {
		return fmt.Errorf("no resumable handler registered for path %q", path)
	}

	// For example, "/operations/" + "create-agent" becomes "/operations/create-agent".
	muxPattern := o.client.muxPrefix + path
	url := o.client.host + muxPattern + "?operation=" + o.row.Operation.GetName()

	if err := o.client.taskQueue.schedulePutRequest(o.Ctx, o.client.mux, url, time.Now().Add(waitDuration)); err != nil {
		_ = o.Fail("scheduling cloudtask: %v", err)
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
func (o *Operation) SaveMetadata(md proto.Message) error {
	if err := o.SetMetadata(md); err != nil {
		return err
	}
	return o.Save()
}

// SetMetadata updates the operation metadata without persisting the operation.
func (o *Operation) SetMetadata(md proto.Message) error {
	mdAny, err := anypb.New(md)
	if err != nil {
		return fmt.Errorf("marshal metadata into any: %w", err)
	}
	o.row.Operation.Metadata = mdAny
	return nil
}

// DecodePrivateState decodes the operation's private state into state.
func (o *Operation) DecodePrivateState(state any) error {
	r := bytes.NewReader(o.row.State)
	dec := gob.NewDecoder(r)
	if err := dec.Decode(state); err != nil {
		return fmt.Errorf("decode private state: %w", err)
	}
	return nil
}

// SavePrivateState updates the private state and persists the operation.
func (o *Operation) SavePrivateState(state any) error {
	if err := o.SetPrivateState(state); err != nil {
		return err
	}
	return o.Save()
}

// SetPrivateState updates the private state without persisting the operation.
func (o *Operation) SetPrivateState(state any) error {
	w := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(w)
	if err := enc.Encode(state); err != nil {
		return fmt.Errorf("encode private state: %w", err)
	}
	o.row.State = w.Bytes()
	return nil
}

// ResumePoint returns the operation's current resume point.
func (o *Operation) ResumePoint() string {
	return o.row.ResumePoint
}

// SaveResumePoint updates the resume point and persists the operation.
func (o *Operation) SaveResumePoint(resumePoint string) error {
	o.SetResumePoint(resumePoint)
	return o.Save()
}

// SetResumePoint updates the resume point without persisting the operation.
func (o *Operation) SetResumePoint(resumePoint string) {
	o.row.ResumePoint = resumePoint
}

// Save persists the current operation row.
func (o *Operation) Save() error {
	if err := o.client.db.Update(o.Ctx, o.row); err != nil {
		return fmt.Errorf("save operation row: %w", err)
	}
	return nil
}

// Complete marks the operation done with a successful response and persists it.
func (o *Operation) Complete(resp proto.Message) error {
	if resp == nil {
		return fmt.Errorf("complete operation %s with nil response", o.row.Operation.GetName())
	}

	respAny, err := anypb.New(resp)
	if err != nil {
		return fmt.Errorf("marshal response into any: %w", err)
	}

	o.row.Operation.Done = true
	o.row.Operation.Result = &longrunningpb.Operation_Response{Response: respAny}
	return o.Save()
}

// Fail marks the operation done with an error message and persists it.
func (o *Operation) Fail(reason string, args ...any) error {
	o.row.Operation.Done = true
	o.row.Operation.Result = &longrunningpb.Operation_Error{
		Error: &rpcStatus.Status{
			Message: fmt.Sprintf(reason, args...),
		},
	}
	return o.Save()
}
