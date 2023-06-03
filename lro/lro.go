package lro

import (
	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"context"
	"fmt"
	"github.com/google/uuid"
	"go.alis.build/alog"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"strings"
)

const ColumnFamily = "0"
const ParentlessOpColumn = "0"

// ErrNotFound is returned when the requested operation does not exist in bigtable
type ErrNotFound struct {
	Operation string // unavailable locations
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.Operation)
}

type InvalidOperationName struct {
	Name string // unavailable locations
}

func (e InvalidOperationName) Error() string {
	return fmt.Sprintf("%s is not a valid operation name (must start with 'operations/')", e.Name)
}

// Client manages the instance of the Bigtable table.
type Client struct {
	table *bigtable.Table
}

// NewClient creates a new lro Client object. The function takes three arguments:
//   - googleProject: The ID of the Google Cloud project that the LroClient object will use.
//   - bigTableInstance: The name of the Bigtable instance that the LroClient object will use.
//   - table: The name of the Bigtable table that the LroClient object will use.
func NewClient(ctx context.Context, googleProject string, bigTableInstance string, table string) *Client {
	client, err := bigtable.NewClient(ctx, googleProject, bigTableInstance)
	if err != nil {
		alog.Fatalf(ctx, "create bigtable client: %s", err)
	}
	return &Client{table: client.Open(table)}
}

type CreateOpts struct {
	id       string
	parent   string
	metadata *anypb.Any
}

// CreateOperation stores a new long-running operation in bigtable, with done=false
func (c *Client) CreateOperation(ctx context.Context, opts CreateOpts) (*longrunningpb.Operation, error) {
	// create new unpopulated long-running operation
	op := &longrunningpb.Operation{}

	// set resource name
	if opts.id == "" {
		opts.id = uuid.New().String()
	}
	op.Name = "operations/" + opts.id

	// set column name
	colName := ParentlessOpColumn
	if opts.parent != "" {
		colName = opts.parent
	}

	//write to bigtable
	err := c.writeToBigtable(ctx, colName, op)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// GetOperation can be used directly in your GetOperation rpc method to return a long-running operation to a client
func (c *Client) GetOperation(ctx context.Context, operationName string) (*longrunningpb.Operation, error) {
	// get operation (ignore column name)
	op, _, err := c.getOpAndColumn(ctx, operationName)
	if err != nil {
		return nil, err
	}
	return op, nil
}

type MetaOptions struct {
	update      bool
	newMetaData proto.Message
}

// SetSuccessful updates an existing long-running operation's done field to true, sets the response and updates the
// metadata if metaOptions.update is true
func (c *Client) SetSuccessful(ctx context.Context, operationName string, response proto.Message, metaOptions MetaOptions) error {
	// get operation and column name
	op, colName, err := c.getOpAndColumn(ctx, operationName)
	if err != nil {
		return err
	}

	// update done and result
	op.Done = true
	resultAny, err := anypb.New(response)
	if err != nil {
		return err
	}
	op.Result = &longrunningpb.Operation_Response{Response: resultAny}

	// update metadata if required
	if metaOptions.update {
		metaAny, err := anypb.New(metaOptions.newMetaData)
		if err != nil {
			return err
		}
		op.Metadata = metaAny
	}

	// write to bigtable
	err = c.writeToBigtable(ctx, colName, op)
	if err != nil {
		return err
	}
	return nil
}

// SetFailed updates an existing long-running operation's done field to true, sets the error and updates the metadata
// if metaOptions.update is true
func (c *Client) SetFailed(ctx context.Context, operationName string, error *status.Status, metaOptions MetaOptions) error {
	// get operation and column name
	op, colName, err := c.getOpAndColumn(ctx, operationName)
	if err != nil {
		return err
	}

	// update operation fields
	op.Done = true
	op.Result = &longrunningpb.Operation_Error{Error: error}
	if metaOptions.update {
		// convert metadata to Any type as per longrunning.Operation requirement.
		metaAny, err := anypb.New(metaOptions.newMetaData)
		if err != nil {
			return err
		}
		op.Metadata = metaAny
	}

	// write to bigtable
	err = c.writeToBigtable(ctx, colName, op)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) writeToBigtable(ctx context.Context, columnName string, op *longrunningpb.Operation) error {
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
	err = c.table.Apply(ctx, operationId, mut)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) getOpAndColumn(ctx context.Context, operation string) (*longrunningpb.Operation, string, error) {

	// validate operation name and get row key
	rowKey, prefixFound := strings.CutPrefix(operation, "operations/")
	if !prefixFound {
		return nil, "", InvalidOperationName{Name: operation}
	}

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
