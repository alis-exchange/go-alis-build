package lro

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/spanner"
	"github.com/google/uuid"
	"go.alis.build/lro/internal/validate"
	"go.alis.build/sproto"
	"golang.org/x/sync/errgroup"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	grpcStatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

/*
SpannerClient support
*/

// Spanner constants
const (
	OperationColumnName  = "Operation"
	ParentColumnName     = "operation_parent"
	OperationTableSuffix = "Operations"
)

type SpannerClient struct {
	client      *sproto.Client
	tableConfig *SpannerTableConfig
}

type SpannerTableConfig struct {
	tableName           string
	operationColumnName string
	parentColumnName    string
}

/*
NewSpannerClient creates a new lro SpannerClient instance for lro management with the provided spanner.Client instance.
*/
func NewSpannerClient(ctx context.Context, productGoogleProject, instanceGoogleProject string, instanceName string, databaseName string, databaseRole string, tableConfig *SpannerTableConfig) (*SpannerClient, error) {
	// Establish sproto spanner connection with fine grained table-level role
	client, err := sproto.NewClient(ctx, instanceGoogleProject, instanceName, databaseName, databaseRole)
	if err != nil {
		return nil, err
	}

	defaultTableConfig := &SpannerTableConfig{
		tableName:           fmt.Sprintf("%s_%s", strings.ReplaceAll(productGoogleProject, "-", "_"), OperationTableSuffix),
		operationColumnName: OperationColumnName,
		parentColumnName:    ParentColumnName,
	}
	// Overwrite defaults where specified
	if tableConfig != nil {
		if tableConfig.tableName != "" {
			defaultTableConfig.tableName = tableConfig.tableName
		}
		if tableConfig.operationColumnName != "" {
			defaultTableConfig.operationColumnName = tableConfig.operationColumnName
		}
		if tableConfig.parentColumnName != "" {
			defaultTableConfig.parentColumnName = tableConfig.parentColumnName
		}
	}

	return &SpannerClient{
		client:      client,
		tableConfig: defaultTableConfig,
	}, nil
}

/*
Close closes the underlying spanner.Client instance.
*/
func (s *SpannerClient) Close() {
	s.client.Close()
}

// CreateOperation stores a new long-running operation in spanner, with done=false
func (s *SpannerClient) CreateOperation(ctx *context.Context, opts *CreateOptions) (*longrunningpb.Operation, error) {
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

	// set metadata
	if opts.Metadata != nil {
		op.Metadata = opts.Metadata
	}

	// {
	// TODO figure out
	// // set column name
	// colName := s.column

	// if opts.Parent != "" {
	// 	colName = opts.Parent
	// } else {
	// 	if incomingMeta, ok := metadata.FromIncomingContext(*ctx); ok {
	// 		if incomingMeta[MetaKeyAlisLroParent] != nil {
	// 			if len(incomingMeta[MetaKeyAlisLroParent]) > 0 {
	// 				colName = incomingMeta[MetaKeyAlisLroParent][0]
	// 			}
	// 		}
	// 	}
	// }
	// }

	parent := ""
	// validate arguments
	if opts.Parent != "" {
		err := validate.Argument("name", opts.Parent, validate.OperationRegex)
		if err != nil {
			return nil, err
		}
		parent = opts.Parent
	}

	// write operation and parent to respective spanner columns
	// rowKey := op.GetName()
	row := map[string]interface{}{
		// NOTE: key is computed as Operation.name
		"Operation":        op,
		"operation_parent": parent,
	}
	err := s.client.InsertRow(*ctx, s.tableConfig.tableName, row)
	if err != nil {
		return nil, err
	}

	// TODO: figure out
	// {
	// add opName to outgoing context metadata
	// *ctx = metadata.AppendToOutgoingContext(*ctx, MetaKeyAlisLroParent, op.GetName())
	// }

	return op, nil
}

// GetOperation can be used directly in your GetOperation rpc method to return a long-running operation to a client
func (s *SpannerClient) GetOperation(ctx context.Context, operationName string) (*longrunningpb.Operation, error) {
	// validate arguments
	err := validate.Argument("name", operationName, validate.OperationRegex)
	if err != nil {
		return nil, err
	}

	// read operation resource from spanner
	op := &longrunningpb.Operation{}
	err = s.client.ReadProto(ctx, s.tableConfig.tableName, spanner.Key{operationName}, s.tableConfig.operationColumnName, op, nil)
	if err != nil {
		if errors.Is(err, sproto.ErrNotFound{}) {
			time.Sleep(5 * time.Millisecond)
			// 5ms back off
			err = s.client.ReadProto(ctx, s.tableConfig.tableName, spanner.Key{operationName}, s.tableConfig.operationColumnName, op, nil)
			if err != nil {
				if errors.Is(err, sproto.ErrNotFound{}) {
					time.Sleep(10 * time.Millisecond)
					// 10ms back off
					err = s.client.ReadProto(ctx, s.tableConfig.tableName, spanner.Key{operationName}, s.tableConfig.operationColumnName, op, nil)
					if err != nil {
						if errors.Is(err, sproto.ErrNotFound{}) {
							time.Sleep(50 * time.Millisecond)
							// 50ms back off
							err = s.client.ReadProto(ctx, s.tableConfig.tableName, spanner.Key{operationName}, s.tableConfig.operationColumnName, op, nil)
							if err != nil {
								if errors.Is(err, sproto.ErrNotFound{}) {
									time.Sleep(100 * time.Millisecond)
									// 100ms back off
									err = s.client.ReadProto(ctx, s.tableConfig.tableName, spanner.Key{operationName}, s.tableConfig.operationColumnName, op, nil)
									if err != nil {
										if errors.Is(err, sproto.ErrNotFound{}) {
											time.Sleep(2 * time.Second)
											// 2s back off
											err = s.client.ReadProto(ctx, s.tableConfig.tableName, spanner.Key{operationName}, s.tableConfig.operationColumnName, op, nil)
											if err != nil {
												if errors.Is(err, sproto.ErrNotFound{}) {
													return nil, status.Errorf(codes.NotFound, "%s not found", operationName)
												}
												return nil, status.Error(codes.Internal, err.Error())
											}
										}
										return nil, status.Error(codes.Internal, err.Error())
									}
								}
								return nil, status.Error(codes.Internal, err.Error())
							}
						}
						return nil, status.Error(codes.Internal, err.Error())
					}
				}
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return op, nil
}

// DeleteOperation deletes the operation row, indexed by operationName, from the spanner table.
func (s *SpannerClient) DeleteOperation(ctx context.Context, operationName string) (*emptypb.Empty, error) {
	// validate operation name
	err := validate.Argument("name", operationName, validate.OperationRegex)
	if err != nil {
		return nil, err
	}

	// validate existence of operation
	_, err = s.GetOperation(ctx, operationName)
	if err != nil {
		return nil, err
	}

	// delete operation
	err = s.client.DeleteRow(ctx, s.tableConfig.tableName, spanner.Key{operationName})
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Errorf("delete operation (%s): %s", operationName, err.Error()).Error())
	}
	return &emptypb.Empty{}, nil
}

// WaitOperation can be used directly in your WaitOperation rpc method to wait for a long-running operation to complete.
// The metadataCallback parameter can be used to handle metadata provided by the operation.
// Note that if you do not specify a timeout, the timeout is set to 49 seconds.
func (s *SpannerClient) WaitOperation(ctx context.Context, req *longrunningpb.WaitOperationRequest, metadataCallback func(*anypb.Any)) (*longrunningpb.Operation, error) {
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
		op, err = s.GetOperation(ctx, req.GetName())
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

		timePassed := time.Since(startTime)
		if timeout != nil && timePassed > duration {
			return nil, ErrWaitDeadlineExceeded{timeout: timeout}
		}
		time.Sleep(1 * time.Second)
	}
}

// BatchWaitOperations is a batch version of the WaitOperation method.
func (s *SpannerClient) BatchWaitOperations(ctx context.Context, operations []*longrunningpb.Operation, timeout *durationpb.Duration) ([]*longrunningpb.Operation, error) {
	// iterate through the requests
	errs, ctx := errgroup.WithContext(ctx)
	results := make([]*longrunningpb.Operation, len(operations))
	for i, operation := range operations {
		i := i
		errs.Go(func() error {
			op, err := s.WaitOperation(ctx, &longrunningpb.WaitOperationRequest{
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
func (s *SpannerClient) SetSuccessful(ctx context.Context, operationName string, response proto.Message, metadata proto.Message) (*longrunningpb.Operation, error) {
	// get operation and parent
	op, parent, err := s.getOperationAndParent(ctx, operationName)
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

	// update in spanner by first deleting and then writing
	_, err = s.DeleteOperation(ctx, op.GetName())
	if err != nil {
		return nil, err
	}

	//  write operation and parent to respective spanner columns
	row := map[string]interface{}{
		// NOTE: key is computed as Operation.name
		"Operation":        op,
		"operation_parent": parent,
	}
	err = s.client.InsertRow(ctx, s.tableConfig.tableName, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// SetFailed updates an existing long-running operation's done field to true, sets the error and updates the metadata
// if metaOptions.Update is true
func (s *SpannerClient) SetFailed(ctx context.Context, operationName string, error error, metadata proto.Message) (*longrunningpb.Operation, error) {
	// get operation and parent name
	op, parent, err := s.getOperationAndParent(ctx, operationName)
	if err != nil {
		return nil, err
	}

	// update operation fields
	op.Done = true
	if error == nil {
		error = grpcStatus.Errorf(codes.Internal, "unknown error")
	}

	stat, ok := status.FromError(error)
	if !ok {
		op.Result = &longrunningpb.Operation_Error{Error: &statuspb.Status{
			Code:    int32(codes.Unknown),
			Message: error.Error(),
			Details: nil,
		}}
	} else {
		op.Result = &longrunningpb.Operation_Error{Error: &statuspb.Status{
			Code:    int32(stat.Code()),
			Message: stat.Message(),
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

	// update in spanner by first deleting and then writing
	_, err = s.DeleteOperation(ctx, op.GetName())
	if err != nil {
		return nil, err
	}

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{
		// NOTE: key is computed as Operation.name
		"Operation":        op,
		"operation_parent": parent,
	}
	err = s.client.InsertRow(ctx, s.tableConfig.tableName, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// UpdateMetadata updates an existing long-running operation's metadata.  Metadata typically
// contains progress information and common metadata such as create time.
func (s *SpannerClient) UpdateMetadata(ctx context.Context, operationName string, metadata proto.Message) (*longrunningpb.Operation, error) {
	// get operation and column name.
	op, parent, err := s.getOperationAndParent(ctx, operationName)
	if err != nil {
		return nil, err
	}

	// update metadata if required
	metaAny, err := anypb.New(metadata)
	if err != nil {
		return nil, err
	}
	op.Metadata = metaAny

	// update in spanner by first deleting and then writing
	_, err = s.DeleteOperation(ctx, op.GetName())
	if err != nil {
		return nil, err
	}

	// write operation and parent to respective spanner columns
	row := map[string]interface{}{
		// NOTE: key is computed as Operation.name
		"Operation":        op,
		"operation_parent": parent,
	}
	err = s.client.InsertRow(ctx, s.tableConfig.tableName, row)
	if err != nil {
		return nil, err
	}

	return op, nil
}

// GetParent attempts to retrieve the parent operation name for a specified operation
func (s *SpannerClient) GetParent(ctx context.Context, operation string) (string, error) {
	// get operation and column name
	_, parent, err := s.getOperationAndParent(ctx, operation)
	if err != nil {
		return "", err
	}
	// return not found error in case parent is empty
	if parent == "" {
		return "", ErrNotFound{Operation: operation}
	}
	return parent, nil
}

// TraverseChildrenOperations returns a tree of all children for a given parent operation
func (s *SpannerClient) TraverseChildrenOperations(ctx context.Context, operation string, opts *TraverseChildrenOperationsOptions) ([]*OperationNode, error) {
	immediateChildren, _, err := s.ListImmediateChildrenOperations(ctx, operation, nil)
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

		children, err := s.TraverseChildrenOperations(ctx, immediateChild.GetName(), &TraverseChildrenOperationsOptions{MaxDepth: newMaxDepth})
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

// ListImmediateChildrenOperations provides the list of immediate children for a given operation
func (s *SpannerClient) ListImmediateChildrenOperations(ctx context.Context, parent string, opts *ListImmediateChildrenOperationsOptions) ([]*longrunningpb.Operation, string, error) {
	// in the case that opts is not provided,
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

	// validate pageToken
	startKey := ""
	if opts.PageToken != "" {
		startKeyBytes, err := base64.StdEncoding.DecodeString(opts.PageToken)
		if err != nil {
			return nil, "", ErrInvalidNextToken{nextToken: opts.PageToken}
		}
		startKey = string(startKeyBytes)
	}

	readOptions := sproto.ReadOptions{
		Limit:     int32(opts.PageSize),
		PageToken: startKey,
	}

	var dml spanner.Statement
	if parent != "" {
		spannerStatement := spanner.NewStatement(fmt.Sprintf("STARTS_WITH(%s, @parent)", ParentColumnName))
		spannerStatement.Params["parent"] = parent
		dml = spannerStatement
	}

	queryRowsMap, lastRowKey, err := s.client.QueryProtos(ctx, s.tableConfig.tableName, []string{s.tableConfig.operationColumnName}, []proto.Message{&longrunningpb.Operation{}}, &dml, &readOptions)
	if err != nil {
		return nil, lastRowKey,
			status.Errorf(codes.Internal, "query protos: %v", err)
	}

	// populate list of ops
	// extract query results into protoreflect.ProtoMessage
	ops := []*longrunningpb.Operation{}
	for _, m := range queryRowsMap {
		if o, ok := m[s.tableConfig.operationColumnName]; ok {
			ops = append(ops, o.(*longrunningpb.Operation))
		}
	}

	// omit first row, which was last row on previous read
	if len(ops) != 0 && opts.PageToken != "" {
		ops = ops[1:]
	}

	if len(ops) < opts.PageSize || len(ops) == 0 {
		lastRowKey = ""
	} else {
		// base64 encode lastRowKey
		lastRowKeyBytes := []byte(lastRowKey)
		lastRowKey = base64.StdEncoding.EncodeToString(lastRowKeyBytes)
	}

	return ops, lastRowKey, nil
}

// getOperationAndParent reads and returns an operation and its parent
func (s *SpannerClient) getOperationAndParent(ctx context.Context, operationName string) (*longrunningpb.Operation, string, error) {
	// validate arguments
	err := validate.Argument("name", operationName, validate.OperationRegex)
	if err != nil {
		return nil, "", err
	}

	// read operation row from spanner
	op := &longrunningpb.Operation{}
	parent := ""
	row, err := s.client.Client().Single().ReadRow(ctx, s.tableConfig.tableName, spanner.Key{operationName}, []string{s.tableConfig.operationColumnName, s.tableConfig.parentColumnName})
	if err != nil {
		return nil, "", err
	}

	// unmarshal values to op and parent
	err = row.ColumnByName(s.tableConfig.operationColumnName, op)
	if err != nil {
		return nil, "", err
	}
	err = row.ColumnByName(s.tableConfig.parentColumnName, &parent)
	if err != nil {
		return nil, "", err
	}

	// return operation and column name
	return op, parent, nil
}
