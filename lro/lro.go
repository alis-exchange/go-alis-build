package lro

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

// Metadata constants
const (
	// MetaKeyAlisLroParent specifies the metadata key name
	// for a long-running operation parent
	MetaKeyAlisLroParent = "x-alis-lro-parent"
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

type ErrInvalidNextToken struct {
	nextToken string
}

func (e ErrInvalidNextToken) Error() string {
	return fmt.Sprintf("invalid nextToken (%s)", e.nextToken)
}

// EOF is the error returned when no more entities (such as children operations)
// are available. Functions should return EOF only to signal a graceful end read.
var EOF = errors.New("EOF")

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
	} else {
		if incomingMeta, ok := metadata.FromIncomingContext(ctx); ok {
			if incomingMeta[MetaKeyAlisLroParent] != nil {
				if len(incomingMeta[MetaKeyAlisLroParent]) > 0 {
					colName = incomingMeta[MetaKeyAlisLroParent][0]
				}
			}
		}
	}

	//write to bigtable
	err := c.writeToBigtable(ctx, c.rowKeyPrefix, colName, op)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// SetOutgoingContextParentOperation appends the outgoing context metadata with the operation name, using the key as MetaKeyAlisLroParent
func (c *Client) SetOutgoingContextParentOperation(ctx context.Context, operationName string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, MetaKeyAlisLroParent, operationName)
}

// GetOperation can be used directly in your GetOperation rpc method to return a long-running operation to a client
func (c *Client) GetOperation(ctx context.Context, operationName string) (*longrunningpb.Operation, error) {
	// get operation (ignore column name)
	op, _, err := c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
	if err != nil {
		// if error is of type ErrNotFound, retry with delay of 5ms, 10ms, 50ms, 100ms, 500ms, 1s, 2s
		if _, ok := err.(ErrNotFound); ok {
			println("operation not found, waiting 1 second and trying again")
			time.Sleep(5 * time.Millisecond)
			op, _, err = c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
			if err != nil {
				if _, ok := err.(ErrNotFound); ok {
					time.Sleep(10 * time.Millisecond)
					op, _, err = c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
					if err != nil {
						if _, ok := err.(ErrNotFound); ok {
							time.Sleep(50 * time.Millisecond)
							op, _, err = c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
							if err != nil {
								if _, ok := err.(ErrNotFound); ok {
									time.Sleep(100 * time.Millisecond)
									op, _, err = c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
									if err != nil {
										if _, ok := err.(ErrNotFound); ok {
											time.Sleep(500 * time.Millisecond)
											op, _, err = c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
											if err != nil {
												if _, ok := err.(ErrNotFound); ok {
													time.Sleep(1 * time.Second)
													op, _, err = c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
													if err != nil {
														if _, ok := err.(ErrNotFound); ok {
															time.Sleep(2 * time.Second)
															op, _, err = c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
															if err != nil {
																return nil, err
															}
														} else {
															return nil, err
														}
													}
												} else {
													return nil, err
												}
											}
										} else {
											return nil, err
										}
									}
								} else {
									return nil, err
								}
							}
						} else {
							return nil, err
						}
					}
				} else {
					return nil, err
				}
			}
		} else {
			return nil, err
		}
	}
	return op, nil
}

// WaitOperation can be used directly in your WaitOperation rpc method to wait for a long-running operation to complete.
// The metadataCallback parameter can be used to handle metadata provided by the operation.
// Note that if you do not specify a timeout, the timeout is set to 49 seconds.
func (c *Client) WaitOperation(ctx context.Context, req *longrunningpb.WaitOperationRequest, metadataCallback func(*anypb.Any)) (*longrunningpb.Operation, error) {
	timeout := req.GetTimeout()
	if timeout == nil {
		timeout = &durationpb.Duration{Seconds: 7 * 7}
	}
	startTime := time.Now()
	duration := time.Duration(timeout.Seconds*1e9 + int64(timeout.Nanos))

	// start loop to check if operation is done or timeout has passed
	var op *longrunningpb.Operation
	var err error
	for {
		op, err = c.GetOperation(ctx, req.GetName())
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

// BatchWaitOperations is a batch version of the WaitOperation method.
func (c *Client) BatchWaitOperations(ctx context.Context, operations []*longrunningpb.Operation, timeout *durationpb.Duration) ([]*longrunningpb.Operation, error) {

	// iterate through the requests
	errs, ctx := errgroup.WithContext(ctx)
	results := make([]*longrunningpb.Operation, len(operations))
	for i, operation := range operations {
		i := i
		errs.Go(func() error {
			op, err := c.WaitOperation(ctx, &longrunningpb.WaitOperationRequest{
				Name:    operation.GetName(),
				Timeout: timeout,
			}, nil)
			if err != nil {
				return err
			}
			results[i] = op

			return nil
		})
	}

	err := errs.Wait()
	if err != nil {
		return nil, err
	}

	return results, nil
}

// SetSuccessful updates an existing long-running operation's done field to true, sets the response and updates the
// metadata if provided.
func (c *Client) SetSuccessful(ctx context.Context, operationName string, response proto.Message, metadata proto.Message) (*longrunningpb.Operation, error) {
	// get operation and column name
	op, colName, err := c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
	if err != nil {
		return nil, err
	}

	// update done and result
	op.Done = true
	if response != nil {
		resultAny, err := anypb.New(response)
		if err != nil {
			return nil, err
		}
		op.Result = &longrunningpb.Operation_Response{Response: resultAny}
	}

	// update metadata if required
	if metadata != nil {
		metaAny, err := anypb.New(metadata)
		if err != nil {
			return nil, err
		}
		op.Metadata = metaAny
	}

	// update in bigtable by first deleting
	err = c.deleteRow(ctx, c.rowKeyPrefix, op.GetName())
	err = c.writeToBigtable(ctx, c.rowKeyPrefix, colName, op)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// SetFailed updates an existing long-running operation's done field to true, sets the error and updates the metadata
// if metaOptions.Update is true
func (c *Client) SetFailed(ctx context.Context, operationName string, error error, metadata proto.Message) (*longrunningpb.Operation, error) {

	// get operation and column name
	op, colName, err := c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
	if err != nil {
		return nil, err
	}

	// update operation fields
	op.Done = true
	if error == nil {
		error = grpcStatus.Errorf(codes.Internal, "unknown error")
	}

	s, ok := status.FromError(error)
	if !ok {
		op.Result = &longrunningpb.Operation_Error{Error: &statuspb.Status{
			Code:    int32(codes.Unknown),
			Message: error.Error(),
			Details: nil,
		}}
	} else {
		op.Result = &longrunningpb.Operation_Error{Error: &statuspb.Status{
			Code:    int32(s.Code()),
			Message: s.Message(),
			Details: nil,
		}}
	}
	if metadata != nil {
		// convert metadata to Any type as per longrunning.Operation requirement.
		metaAny, err := anypb.New(metadata)
		if err != nil {
			return nil, err
		}
		op.Metadata = metaAny
	}

	// write to bigtable by first deleting
	err = c.deleteRow(ctx, c.rowKeyPrefix, op.GetName())
	err = c.writeToBigtable(ctx, c.rowKeyPrefix, colName, op)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// UpdateMetadata updates an existing long-running operation's metadata.  Metadata typically
// contains progress information and common metadata such as create time.
func (c *Client) UpdateMetadata(ctx context.Context, operationName string, metadata proto.Message) (*longrunningpb.Operation, error) {
	// get operation and column name.
	op, colName, err := c.getOpAndColumn(ctx, c.rowKeyPrefix, operationName)
	if err != nil {
		return nil, err
	}

	// update metadata if required
	metaAny, err := anypb.New(metadata)
	if err != nil {
		return nil, err
	}
	op.Metadata = metaAny

	// update in bigtable by first deleting
	err = c.deleteRow(ctx, c.rowKeyPrefix, op.GetName())
	err = c.writeToBigtable(ctx, c.rowKeyPrefix, colName, op)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// GetParent attempts to retrieve the parent operation name for a specified operation
func (c *Client) GetParent(ctx context.Context, operation string) (string, error) {
	// get operation and column name
	_, colName, err := c.getOpAndColumn(ctx, c.rowKeyPrefix, operation)
	if err != nil {
		return "", err
	}
	// if colName is ParentlessOpColumn, return not found error
	if colName == ParentlessOpColumn {
		return "", ErrNotFound{Operation: operation}
	}
	return colName, nil
}

// TraverseChildrenOperationsOptions is an optional parameter that
// can be provided to TraverseChildrenOperations
type TraverseChildrenOperationsOptions struct {
	// The maximum depth of the tree to return. If not specified, the entire tree is returned.
	MaxDepth int
}

// OperationNode provides a data structure to represent
// the relationship between an operation and potential
// children operations.
type OperationNode struct {
	// The name of the operation.
	//
	// In the case that ChildrenOperations
	// exist, Operation represents a parent
	// operation that can be further traversed.
	// Else, the operation is the last node in
	// the overall operation tree.
	Operation string
	// The set of children operations to
	// the Operation.
	ChildrenOperations []*OperationNode
}

// TraverseChildrenOperations returns a tree of all children for a given parent operation
func (c *Client) TraverseChildrenOperations(ctx context.Context, operation string, opts *TraverseChildrenOperationsOptions) ([]*OperationNode, error) {
	immediateChildren, _, err := c.ListImmediateChildrenOperations(ctx, operation, nil)
	if err != nil {
		return nil, err
	}
	if opts == nil {
		opts = &TraverseChildrenOperationsOptions{
			MaxDepth: 0,
		}
	}
	if len(immediateChildren) == 0 || opts.MaxDepth == -1 {
		// This signals the end of the sequence.
		// Ie. that the OperationNode does not
		// have any children.
		return nil, EOF
	}

	var res []*OperationNode
	for _, immediateChild := range immediateChildren {
		child := &OperationNode{
			Operation: immediateChild.GetName(),
		}

		// opts.MaxDepth = 0 indicates that there is no max depth, thus when 0 is reached, change it to -1 to indicate the end of the sequence
		newMaxDepth := opts.MaxDepth
		if opts.MaxDepth != 0 {
			newMaxDepth = opts.MaxDepth - 1
			if newMaxDepth == 0 {
				newMaxDepth = -1
			}
		}

		children, err := c.TraverseChildrenOperations(ctx, immediateChild.GetName(), &TraverseChildrenOperationsOptions{MaxDepth: newMaxDepth})
		if err != nil {
			if !errors.Is(err, EOF) {
				return nil, err
			}
		}
		child.ChildrenOperations = children
		res = append(res, child)
	}
	return res, nil
}

// ListImmediateChildrenOperationsOptions is an optional parameter that
// can be provided to ListImmediateChildrenOperations
type ListImmediateChildrenOperationsOptions struct {
	// The maximum number of children operations to return.
	// If not specified, the entire list is returned.
	PageSize int
	// A page token, received from a previous ListImmediateChildrenOperations call.
	// If not specified, the first page of results is returned.
	//
	// When paginating, all other parameters provided to `ListBooks` must match
	// the call that provided the page token.
	PageToken string
}

// ListImmediateChildrenOperations provides the list of immediate children for a given operation
func (c *Client) ListImmediateChildrenOperations(ctx context.Context, parent string, opts *ListImmediateChildrenOperationsOptions) ([]*longrunningpb.Operation, string, error) {

	// In the case that opts is not provided,
	// create an empty opts object that will
	// be used in the later aspects of the function
	if opts == nil {
		opts = &ListImmediateChildrenOperationsOptions{
			PageSize:  0,
			PageToken: "",
		}
	} else if opts.PageToken != "" && opts.PageSize != 0 {
		// increase page size by one if nextToken is set, because the nextToken is the rowKey of the last row returned in
		// the previous response, and thus the first element returned in this response will be ignored
		opts.PageSize++
	}

	// set up reading options
	var readingOpts []bigtable.ReadOption
	if opts.PageSize != 0 {
		readingOpts = append(readingOpts, bigtable.LimitRows(int64(opts.PageSize)))
	}
	readingOpts = append(readingOpts, bigtable.RowFilter(bigtable.ChainFilters(
		bigtable.LatestNFilter(1),
		bigtable.ColumnFilter(parent),
	)))

	// create a rowSet to read from
	startKey := ""
	if opts.PageToken != "" {
		startKeyBytes, err := base64.StdEncoding.DecodeString(opts.PageToken)
		if err != nil {
			return nil, "", ErrInvalidNextToken{nextToken: opts.PageToken}
		}
		startKey = string(startKeyBytes)
	}
	endKey := "~~~~~~~~~~~~~~~~~~~~~~~~~~~"
	rowSet := bigtable.NewRange(startKey, endKey)

	lastRowKey := ""
	var res []*longrunningpb.Operation
	err := c.table.ReadRows(ctx, rowSet,
		func(row bigtable.Row) bool {

			// if the row is empty, append an empty value and continue
			if row == nil {
				res = append(res, nil)
				return true
			}

			// Each collection is stored in a corresponding Bigtable family
			columns := row[ColumnFamily]

			// if there are no results in the row, append an empty value and continue
			if len(columns) == 0 {
				res = append(res, nil)
				return true
			}

			// only the first column is used by the resource.
			column := columns[0]
			// unmarshal column.value into long-running operation resource
			op := &longrunningpb.Operation{}
			err := proto.Unmarshal(column.Value, op)
			if err != nil {
				return false
			}
			res = append(res, op)
			lastRowKey = row.Key()
			return true
		},
		readingOpts...,
	)
	if err != nil {
		return nil, lastRowKey, err
	}
	if len(res) != 0 && opts.PageToken != "" {
		res = res[1:]
	}

	if len(res) < opts.PageSize || len(res) == 0 {
		lastRowKey = ""
	} else {
		// base64 encode lastRowKey
		lastRowKeyBytes := []byte(lastRowKey)
		lastRowKey = base64.StdEncoding.EncodeToString(lastRowKeyBytes)
	}
	return res, lastRowKey, nil
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
	columnName := strings.Split(column.Column, ":")[1]
	// return operation and column name
	return op, columnName, nil
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
