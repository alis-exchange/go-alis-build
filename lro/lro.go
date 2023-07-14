package lro

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Bigtable constants
const (
	ColumnFamily       = "0"
	ParentlessOpColumn = "0"
)

// ErrNotFound is returned when the requested operation does not exist in bigtable
type ErrNotFound struct {
	Operation string // unavailable locations
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.Operation)
}

// ErrWaitDeadlineExceeded is returned when the WaitOperation exceeds the specified, or default, timeout
type ErrWaitDeadlineExceeded struct {
	timeout *durationpb.Duration
}

func (e ErrWaitDeadlineExceeded) Error() string {
	return fmt.Sprintf("exceeded timeout deadline of %d seconds", e.timeout.GetSeconds())
}

type InvalidOperationName struct {
	Name string // unavailable locations
}

func (e InvalidOperationName) Error() string {
	return fmt.Sprintf("%s is not a valid operation name (must start with 'operations/')", e.Name)
}

// Client manages the instance of the Bigtable table.
type Client struct {
	table        *bigtable.Table
	rowKeyPrefix string
}

// NewClient creates a new lro Client object. The function takes three arguments:
//   - googleProject: The ID of the Google Cloud project that the LroClient object will use.
//   - bigTableInstance: The name of the Bigtable instance that the LroClient object will use.
//   - table: The name of the Bigtable table that the LroClient object will use.
//   - rowKeyPrefix: This should be an empty string if you have a dedicated table for long-running ops, but if you are
//     sharing a table, the rowKeyPrefix can be used to separate your long-op data from other data in the table
func NewClient(ctx context.Context, googleProject string, bigTableInstance string, table string, rowKeyPrefix string) (*Client, error) {
	client, err := bigtable.NewClient(ctx, googleProject, bigTableInstance)
	if err != nil {
		return nil, fmt.Errorf("create bigtable client: %w", err)
	}
	return &Client{table: client.Open(table), rowKeyPrefix: rowKeyPrefix}, nil
}

// CreateOptions provide additional, optional, parameters to the CreateOperation method.
type CreateOptions struct {
	Id       string     // Id is used to provide user defined operation Ids
	Parent   string     // Parent is the parent long-running operation responsible for creating the new LRO.  Format should be operations/*
	Metadata *anypb.Any // Metadata object as defined for the relevant LRO metadata response.
}

// CreateOperation stores a new long-running operation in bigtable, with done=false
func (c *Client) CreateOperation(ctx context.Context, opts *CreateOptions) (*longrunningpb.Operation, error) {
	// create new unpopulated long-running operation
	op := &longrunningpb.Operation{}

	// set opts to empty struct if nil
	if opts == nil {
		opts = &CreateOptions{}
	}

	// set resource name
	if opts.Id == "" {
		opts.Id = uuid.New().String()
	}
	op.Name = "operations/" + opts.Id

	// set column name
	colName := ParentlessOpColumn
	if opts.Parent != "" {
		colName = opts.Parent
	}

	//write to bigtable
	err := c.writeToBigtable(ctx, c.rowKeyPrefix, colName, op)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// GetOperation can be used directly in your GetOperation rpc method to return a long-running operation to a client
func (c *Client) GetOperation(ctx context.Context, operationName string) (*longrunningpb.Operation, error) {
	// get operation (ignore column name)
	op, _, err := c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// WaitOperation can be used directly in your WaitOperation rpc method to wait for a long-running operation to complete. The metadataCallback parameter can be used to handle metadata provided by the operation. Note that if you do not specify a timeout, the timeout is set to 15 seconds.
func (c *Client) WaitOperation(ctx context.Context, req *longrunningpb.WaitOperationRequest, metadataCallback func(*anypb.Any)) (*longrunningpb.Operation, error) {
	timeout := req.GetTimeout()
	if timeout == nil {
		timeout = &durationpb.Duration{Seconds: 15}
	}
	startTime := time.Now()
	duration := time.Duration(timeout.Seconds*1e9 + int64(timeout.Nanos))

	// start loop to check if operation is done or timeout has passed
	for {
		op, err := c.GetOperation(ctx, req.GetName())
		if err != nil {
			return nil, err
		}
		if op.Done {
			return op, nil
		}
		if metadataCallback != nil && op.Metadata != nil {
			// If a metadata callback was provided, and metadata is available, pass the metadata to the callback.
			metadataCallback(op.GetMetadata())
		}

		timePassed := time.Now().Sub(startTime)
		if timeout != nil && timePassed > duration {
			return nil, ErrWaitDeadlineExceeded{timeout: timeout}
		}
		time.Sleep(1 * time.Second)
	}
}

// SetSuccessful updates an existing long-running operation's done field to true, sets the response and updates the
// metadata if provided.
func (c *Client) SetSuccessful(ctx context.Context, operationName string, response proto.Message, metadata proto.Message) error {
	// get operation and column name
	op, colName, err := c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
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

	// update metadata if required
	if metadata != nil {
		metaAny, err := anypb.New(metadata)
		if err != nil {
			return err
		}
		op.Metadata = metaAny
	}

	// update in bigtable by first deleting
	err = c.deleteRow(ctx, c.rowKeyPrefix, op.GetName())
	err = c.writeToBigtable(ctx, c.rowKeyPrefix, colName, op)
	if err != nil {
		return err
	}
	return nil
}

// SetFailed updates an existing long-running operation's done field to true, sets the error and updates the metadata
// if metaOptions.Update is true
func (c *Client) SetFailed(ctx context.Context, operationName string, error error, metadata proto.Message) error {

	// get operation and column name
	op, colName, err := c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
	if err != nil {
		return err
	}

	// update operation fields
	op.Done = true
	if error == nil {
		error = grpcStatus.Errorf(codes.Internal, "unknown error")
	}
	op.Result = &longrunningpb.Operation_Error{Error: &status.Status{
		Code:    int32(grpcStatus.Code(err)),
		Message: error.Error(),
		Details: nil,
	}}
	if metadata != nil {
		// convert metadata to Any type as per longrunning.Operation requirement.
		metaAny, err := anypb.New(metadata)
		if err != nil {
			return err
		}
		op.Metadata = metaAny
	}

	// write to bigtable by first deleting
	err = c.deleteRow(ctx, c.rowKeyPrefix, op.GetName())
	err = c.writeToBigtable(ctx, c.rowKeyPrefix, colName, op)
	if err != nil {
		return err
	}
	return nil
}

// UpdateMetadata updates an existing long-running operation's metadata.  Metadata typically
// contains progress information and common metadata such as create time.
func (c *Client) UpdateMetadata(ctx context.Context, operationName string, metadata proto.Message) error {
	// get operation and column name.
	op, colName, err := c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
	if err != nil {
		return err
	}

	// update metadata if required
	metaAny, err := anypb.New(metadata)
	if err != nil {
		return err
	}
	op.Metadata = metaAny

	// update in bigtable by first deleting
	err = c.deleteRow(ctx, c.rowKeyPrefix, op.GetName())
	err = c.writeToBigtable(ctx, c.rowKeyPrefix, colName, op)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) writeToBigtable(ctx context.Context, rowKeyPrefix string, columnName string, op *longrunningpb.Operation) error {
	// marshal proto into bytes
	dataBytes, err := proto.Marshal(op)
	if err != nil {
		return err
	}

	// create mutation
	mut := bigtable.NewMutation()
	mut.Set(ColumnFamily, columnName, bigtable.Now(), dataBytes)

	// apply mutation
	operationId, _ := strings.CutPrefix(op.Name, "operations/")
	rowKey := rowKeyPrefix + operationId
	err = c.table.Apply(ctx, rowKey, mut)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) getOpAndColumn(ctx context.Context, rowKeyPrefix, operation string) (*longrunningpb.Operation, string, error) {

	// validate operation name and get row key
	operationId, prefixFound := strings.CutPrefix(operation, "operations/")
	if !prefixFound {
		return nil, "", InvalidOperationName{Name: operation}
	}
	rowKey := rowKeyPrefix + operationId

	// read row from bigtable
	filter := bigtable.ChainFilters(bigtable.LatestNFilter(1), bigtable.FamilyFilter(ColumnFamily))
	row, err := c.table.ReadRow(ctx, rowKey, bigtable.RowFilter(filter))
	if err != nil {
		return nil, "", err
	}
	if row == nil {
		return nil, "", ErrNotFound{Operation: operation}
	}

	//get column (only the first one is used)
	columns, ok := row[ColumnFamily]
	if !ok {
		return nil, "", ErrNotFound{Operation: operation}
	}
	if len(columns) == 0 {
		return nil, "", ErrNotFound{Operation: operation}
	}
	column := columns[0]

	// unmarshal bytes into long-running operation resource
	op := &longrunningpb.Operation{}
	err = proto.Unmarshal(column.Value, op)
	if err != nil {
		return nil, "", err
	}

	// return operation and column name
	return op, column.Column, nil
}

// DeleteRow deletes an entire row from bigtable at the given rowKey.
func (c *Client) deleteRow(ctx context.Context, rowKeyPrefix string, operation string) error {
	// validate operation name and get row key
	operationId, prefixFound := strings.CutPrefix(operation, "operations/")
	if !prefixFound {
		return InvalidOperationName{Name: operation}
	}
	rowKey := rowKeyPrefix + operationId

	// Create a single mutation to delete the row
	mut := bigtable.NewMutation()
	mut.DeleteRow()
	err := c.table.Apply(ctx, rowKey, mut)
	if err != nil {
		return fmt.Errorf("delete bigtable row: %w", err)
	}
	return nil
}
