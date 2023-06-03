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

type LroClient struct {
	t *bigtable.Table
}

// NewClient creates a new LroClient
func NewClient(ctx context.Context, googleProject string, bigTableInstance string, table string) *LroClient {
	client, err := bigtable.NewClient(ctx, googleProject, bigTableInstance)
	if err != nil {
		alog.Fatalf(ctx, "Error creating bigtable client: %s", err)
	}
	return &LroClient{t: client.Open(table)}
}

type CreateOpts struct {
	id       string
	parent   string
	metadata *anypb.Any
}

// CreateOperation stores a new long-running operation in bigtable, with done=false
func (l *LroClient) CreateOperation(ctx context.Context, opts CreateOpts) (*longrunningpb.Operation, error) {
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
	err := l.writeToBigtable(ctx, colName, op)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// GetOperation can be used directly in your GetOperation rpc method to return a long-running operation to a client
func (l *LroClient) GetOperation(ctx context.Context, operationName string) (*longrunningpb.Operation, error) {
	// get operation (ignore column name)
	op, _, err := l.getOpAndColumn(ctx, operationName)
	if err != nil {
		return nil, err
	}
	return op, nil
}

type MetaOptions struct {
	update      bool
	newMetaData proto.Message
}

// SetSuccessful updates an existing long-running operation's done field to true, sets the response and updates the metadata if metaOptions.update is true
func (l *LroClient) SetSuccessful(ctx context.Context, operationName string, response proto.Message, metaOptions MetaOptions) error {
	// get operation and column name
	op, colName, err := l.getOpAndColumn(ctx, operationName)
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
	err = l.writeToBigtable(ctx, colName, op)
	if err != nil {
		return err
	}
	return nil
}

// SetFailed updates an existing long-running operation's done field to true, sets the error and updates the metadata if metaOptions.update is true
func (l *LroClient) SetFailed(ctx context.Context, operationName string, error *status.Status, metaOptions MetaOptions) error {
	// get operation and column name
	op, colName, err := l.getOpAndColumn(ctx, operationName)
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
	err = l.writeToBigtable(ctx, colName, op)
	if err != nil {
		return err
	}
	return nil
}

func (l *LroClient) writeToBigtable(ctx context.Context, columnName string, op *longrunningpb.Operation) error {
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
	err = l.t.Apply(ctx, operationId, mut)
	if err != nil {
		return err
	}
	return nil
}

func (l *LroClient) getOpAndColumn(ctx context.Context, operation string) (*longrunningpb.Operation, string, error) {

	// validate operation name and get row key
	rowKey, prefixFound := strings.CutPrefix(operation, "operations/")
	if !prefixFound {
		return nil, "", InvalidOperationName{Name: operation}
	}

	// read row from bigtable
	filter := bigtable.ChainFilters(bigtable.LatestNFilter(1), bigtable.FamilyFilter(ColumnFamily))
	row, err := l.t.ReadRow(ctx, rowKey, bigtable.RowFilter(filter))
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
