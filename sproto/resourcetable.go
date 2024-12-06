package sproto

import (
	"context"
	"fmt"
	"io"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner"
	"github.com/mennanov/fmutils"
	"go.alis.build/alog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type ResourceTblOptions struct {
	// whether the resource is a version, i.e. the name has the format .../versions/%d-%d-%d
	// If set, the conversion from the resource name to the row key will be such that
	// the latest version is returned first when doing a query (which orders keys lexicographically)
	IsVersion bool
	// The default limit for queries if not provided in QueryOptions. If not provided, 100 is used.
	DefaultLimit int
	// Whether this resource type has iam policies stored next to it. If true, the ResourceRow's policy field
	// will be populated with the policy for the resource when doing a read.
	HasIamPolicy bool
	// Whether to return permission denied for not found resources instead of not found error
	// when doing a read or delete operation.
	ReturnPermissionDeniedForNotFound bool
	// The name of the column that contains the row key in the table. If not provided, 'key' is used.
	KeyColumnName string
	// Whether the table only has standard columns, i.e.
	// - the primary key column called "key"
	// - a proto column with the same name as the last part of the proto message (e.g. "Time" for "google.protobuf.Time")
	// - a proto column with the name "Policy" for the google.iam.v1.Policy message
	// If true, startup would be faster since the columns do not need to be fetched from the database to setup the client.
	StandardColumnsOnly bool
}

/*
ResourceClient is a client for a resource table. It provides methods for creating, reading, updating, and deleting resources and
policies.
*/
type ResourceClient struct {
	tbl                               *TableClient
	RowKeyConv                        *RowKeyConverter
	resourceMsg                       proto.Message
	hasIamPolicy                      bool
	returnPermissionDeniedForNotFound bool
	keyColumnName                     string
}

/*
ResourceRow represents a row in a resource table. It contains the row key, the resource(proto.Message), and the policy(*iampb.Policy).
*/
type ResourceRow struct {
	// The row key of the resource
	RowKey string
	// The resource itself as a proto.Message. This can be cast to the appropriate message type.
	Resource proto.Message
	// The IAM policy for the resource. This will be nil if the resource does not have IAM policies.
	Policy         *iampb.Policy
	resourceClient *ResourceClient
}

// Set the row key for the row. This is needed if a list/query/stream was run and any of the rows returned by the query needs to be updated/deleted.
func (rr *ResourceRow) SetRowKey(name string) error {
	rowKey, err := rr.resourceClient.RowKeyConv.GetRowKey(name)
	if err != nil {
		return err
	}
	rr.RowKey = rowKey
	return nil
}

/*
Merge merges the updatedMsg into the resource. The fieldMaskPaths are the paths of the fields to update.
*/
func (rr *ResourceRow) Merge(updatedMsg proto.Message, fieldMaskPaths ...string) {
	clonedUpdatedMsg := proto.Clone(updatedMsg)
	fmutils.Filter(clonedUpdatedMsg, fieldMaskPaths)
	fmutils.Prune(rr.Resource, fieldMaskPaths)
	proto.Merge(rr.Resource, clonedUpdatedMsg)
}

/*
Update the resource in the database. Does not update the policy.

This method may return a ErrNotFound error if the row does not exist in the table.
*/
func (rr *ResourceRow) Update(ctx context.Context) error {
	if rr.RowKey == "" {
		return status.Error(codes.InvalidArgument, "Row key is empty because row was retrieved via Query,List or Stream method. Use SetRowKey to set the row key")
	}
	return rr.resourceClient.tbl.Update(ctx, spanner.Key{rr.RowKey}, rr.Resource)
}

/*
Delete deletes the resource from the database.
*/
func (rr *ResourceRow) Delete(ctx context.Context) error {
	if rr.RowKey == "" {
		return status.Error(codes.InvalidArgument, "Row key is empty because row was retrieved via Query,List or Stream method. Use SetRowKey to set the row key")
	}
	return rr.resourceClient.tbl.Delete(ctx, spanner.Key{rr.RowKey})
}

/*
NewResourceClient creates a new ResourceClient for the given table name and resource message.
*/
func (d *DbClient) NewResourceClient(tableName string, msg proto.Message, options *ResourceTblOptions) *ResourceClient {
	// get msg type using protoreflect
	msgType := msg.ProtoReflect().Type()
	msgName := string(msgType.Descriptor().FullName())
	shortName := string(msgType.Descriptor().Name())

	if options == nil {
		options = &ResourceTblOptions{}
	}
	if options.DefaultLimit == 0 {
		options.DefaultLimit = 100
	}
	if options.KeyColumnName == "" {
		options.KeyColumnName = "key"
	}
	tblOptions := []TableClientOption{}
	if options.StandardColumnsOnly {
		tblOptions = append(tblOptions, WithPrimaryKeyColumns([]*primaryKeyColumn{
			{
				columnName:  "key",
				isGenerated: false,
				isStored:    false,
			},
		}))
		tblOptions = append(tblOptions, WithMsgTypeToColumnMap(map[string]string{
			msgName:                shortName,
			"google.iam.v1.Policy": "Policy",
		}))
	}
	tableClient, err := d.NewTableClient(tableName, options.DefaultLimit, tblOptions...)
	if err != nil {
		// TODO: Return error instead.
		//  I strongly believe the user should decide on how to handle the error.
		alog.Fatalf(context.Background(), "Failed to create table client: %v", err)
	}
	rt := &ResourceClient{
		tbl:                               tableClient,
		resourceMsg:                       msg,
		RowKeyConv:                        &RowKeyConverter{AbbreviateCollectionIdentifiers: true, LatestVersionFirst: options.IsVersion},
		hasIamPolicy:                      options.HasIamPolicy,
		returnPermissionDeniedForNotFound: options.ReturnPermissionDeniedForNotFound,
		keyColumnName:                     options.KeyColumnName,
	}
	return rt
}

/*
Client returns the underlying spanner.Client instance.
This client can be used to perform custom queries and mutations.
*/
func (rt *ResourceClient) Client() *spanner.Client {
	return rt.tbl.db.client
}

func (rt *ResourceClient) Create(ctx context.Context, name string, resource proto.Message, policy *iampb.Policy) (*ResourceRow, error) {
	if policy == nil && rt.hasIamPolicy {
		return nil, status.Error(codes.InvalidArgument, "Policy required because resource type has iam policies")
	} else if policy != nil && !rt.hasIamPolicy {
		return nil, status.Error(codes.InvalidArgument, "Policy not allowed because resource type does not have iam policies")
	}
	rowKey, err := rt.RowKeyConv.GetRowKey(name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to convert resource name to row key: %v", err)
	}
	msgs := []proto.Message{resource}
	if rt.hasIamPolicy {
		msgs = append(msgs, policy)
	}
	err = rt.tbl.Create(ctx, spanner.Key{rowKey}, msgs...)
	if err != nil {
		return nil, err
	}
	resourceRow := &ResourceRow{
		RowKey:         rowKey,
		Resource:       resource,
		resourceClient: rt,
	}
	if rt.hasIamPolicy {
		if policy.Etag == nil {
			return nil, status.Error(codes.InvalidArgument, "Policy etag is required")
		}
		resourceRow.Policy = policy
	}
	return resourceRow, nil
}

func (rt *ResourceClient) BatchCreate(ctx context.Context, names []string, resources []proto.Message, policies []*iampb.Policy) ([]*ResourceRow, error) {
	if len(resources) != len(names) {
		return nil, status.Error(codes.InvalidArgument, "names and resources must be of the same length")
	}
	if rt.hasIamPolicy && len(policies) != len(names) {
		return nil, status.Error(codes.InvalidArgument, "policies must be of the same length as names")
	}
	rows := []*Row{}
	for i, name := range names {
		rowKey, err := rt.RowKeyConv.GetRowKey(name)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to convert resource name to row key: %v", err)
		}
		row := &Row{
			Key: spanner.Key{rowKey},
			Messages: []proto.Message{
				resources[i],
			},
		}
		if rt.hasIamPolicy {
			row.Messages = append(row.Messages, policies[i])
		}
		rows = append(rows, row)
	}

	err := rt.tbl.BatchCreate(ctx, rows)
	if err != nil {
		return nil, err
	}
	resourceRows := make([]*ResourceRow, len(names))
	for i, row := range rows {
		resourceRow := &ResourceRow{
			RowKey:         row.Key.String(),
			Resource:       row.Messages[0],
			resourceClient: rt,
		}
		if rt.hasIamPolicy {
			resourceRow.Policy = row.Messages[1].(*iampb.Policy)
		}
		resourceRows[i] = resourceRow
	}
	return resourceRows, nil
}

func (rt *ResourceClient) Read(ctx context.Context, name string, fieldMaskPaths ...string) (*ResourceRow, error) {
	rowKey, err := rt.RowKeyConv.GetRowKey(name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to convert resource name to row key: %v", err)
	}
	msg := proto.Clone(rt.resourceMsg)
	msgs := []proto.Message{msg}
	policy := &iampb.Policy{}
	if rt.hasIamPolicy {
		msgs = append(msgs, policy)
	}
	err = rt.tbl.ReadWithFieldMask(ctx, spanner.Key{rowKey}, msgs, []*fieldmaskpb.FieldMask{{Paths: fieldMaskPaths}})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			if rt.returnPermissionDeniedForNotFound {
				return nil, status.Errorf(codes.PermissionDenied, "you do not have the required permission to access this resource or it does not exist")
			} else {
				return nil, status.Errorf(codes.NotFound, "%s not found", name)
			}
		}
		return nil, err
	}
	resourceRow := &ResourceRow{
		RowKey:         rowKey,
		Resource:       msg,
		resourceClient: rt,
	}
	if rt.hasIamPolicy {
		resourceRow.Policy = policy
	}
	return resourceRow, nil
}

func (rt *ResourceClient) UpdatePolicy(ctx context.Context, name string, policy *iampb.Policy) error {
	if !rt.hasIamPolicy {
		return status.Error(codes.InvalidArgument, "Policy not allowed because resource type does not have iam policies")
	}
	if policy == nil {
		return status.Error(codes.InvalidArgument, "Policy is nil")
	}
	if policy.Etag == nil {
		return status.Error(codes.InvalidArgument, "Policy etag is required")
	}
	rowKey, err := rt.RowKeyConv.GetRowKey(name)
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to convert resource name to row key: %v", err)
	}
	return rt.tbl.Update(ctx, spanner.Key{rowKey}, policy)
}

// Batch read resources. The names must be of the same resource type.
func (rt *ResourceClient) BatchRead(ctx context.Context, names []string, fieldMaskPaths ...string) ([]*ResourceRow, []string, error) {
	rowKeys := make([]spanner.Key, len(names))
	for i, name := range names {
		rowKey, err := rt.RowKeyConv.GetRowKey(name)
		if err != nil {
			return nil, nil, status.Errorf(codes.Internal, "Failed to convert resource name to row key: %v", err)
		}
		rowKeys[i] = spanner.Key{rowKey}
	}
	msgs := []proto.Message{proto.Clone(rt.resourceMsg)}
	if rt.hasIamPolicy {
		msgs = append(msgs, &iampb.Policy{})
	}
	rows, err := rt.tbl.BatchReadWithFieldMask(ctx, rowKeys, msgs, []*fieldmaskpb.FieldMask{{Paths: fieldMaskPaths}})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			if rt.returnPermissionDeniedForNotFound {
				return nil, nil, status.Errorf(codes.PermissionDenied, "you do not have the required permission to access one of these resources or it does not exist")
			} else {
				return nil, nil, status.Errorf(codes.NotFound, "one of the resources not found")
			}
		}
		return nil, nil, err
	}
	resourceRows := []*ResourceRow{}
	notFound := []string{}
	for i, row := range rows {
		if row == nil {
			notFound = append(notFound, names[i])
			continue
		}
		key, ok := rowKeys[i][0].(string)
		if !ok {
			return nil, nil, status.Errorf(codes.Internal, "Failed to convert row key to string: %v", err)
		}
		resourceRow := &ResourceRow{
			RowKey:         key,
			Resource:       row.Messages[0],
			resourceClient: rt,
		}
		if rt.hasIamPolicy {
			resourceRow.Policy = row.Messages[1].(*iampb.Policy)
		}
		resourceRows = append(resourceRows, resourceRow)
	}
	return resourceRows, notFound, nil
}

func (rt *ResourceClient) List(ctx context.Context, parent string, filter *spanner.Statement, opts *QueryOptions) ([]*ResourceRow, string, error) {
	var err error
	var spannerStatement spanner.Statement
	if parent != "" {
		spannerStatement = spanner.NewStatement(fmt.Sprintf("STARTS_WITH(%s,@prefix)", rt.keyColumnName))
		spannerStatement.Params["prefix"], err = rt.RowKeyConv.GetRowKeyPrefix(parent)
		if err != nil {
			return nil, "", status.Errorf(codes.Internal, "Failed to convert parent name to row key prefix: %v", err)
		}
		// Append filter to the statement
		if filter != nil {
			spannerStatement.SQL += " AND " + filter.SQL
			for k, v := range filter.Params {
				spannerStatement.Params[k] = v
			}
		}
	} else {
		// Append filter to the statement
		if filter != nil {
			spannerStatement = spanner.Statement{
				SQL:    filter.SQL,
				Params: filter.Params,
			}
		}
	}

	msgs := []proto.Message{proto.Clone(rt.resourceMsg)}
	if rt.hasIamPolicy {
		msgs = append(msgs, &iampb.Policy{})
	}
	rows, nextToken, err := rt.tbl.Query(ctx, msgs, &spannerStatement, opts)
	if err != nil {
		return nil, "", err
	}
	resourceRows := make([]*ResourceRow, len(rows))
	for i, row := range rows {
		resourceRow := &ResourceRow{
			Resource:       row.Messages[0],
			resourceClient: rt,
		}
		if rt.hasIamPolicy {
			resourceRow.Policy = row.Messages[1].(*iampb.Policy)
		}
		resourceRows[i] = resourceRow
	}
	return resourceRows, nextToken, nil
}

func (rt *ResourceClient) Stream(ctx context.Context, parent string, opts *QueryOptions) (*StreamResponse[ResourceRow], error) {
	var err error
	var spannerStatement spanner.Statement
	if parent != "" {
		spannerStatement = spanner.NewStatement(fmt.Sprintf("STARTS_WITH(%s,@prefix)", rt.keyColumnName))
		spannerStatement.Params["prefix"], err = rt.RowKeyConv.GetRowKeyPrefix(parent)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to convert parent name to row key prefix: %v", err)
		}
	}

	msgs := []proto.Message{proto.Clone(rt.resourceMsg)}
	if rt.hasIamPolicy {
		msgs = append(msgs, &iampb.Policy{})
	}
	var tblOpts *StreamOptions
	if opts != nil {
		tblOpts = &StreamOptions{
			SortColumns: opts.SortColumns,
			Limit:       opts.Limit,
			ReadMasks:   opts.ReadMasks,
		}
	}

	res := NewStreamResponse[ResourceRow]()
	go func() {
		ctx := context.WithoutCancel(ctx)
		it, err := rt.tbl.Stream(ctx, msgs, &spannerStatement, tblOpts)
		if err != nil {
			res.setError(err)
			return
		}

		for {
			row, err := it.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				res.setError(err)
				return
			}

			resourceRow := &ResourceRow{
				Resource:       row.Messages[0],
				resourceClient: rt,
			}
			if rt.hasIamPolicy {
				resourceRow.Policy = row.Messages[1].(*iampb.Policy)
			}

			res.addItem(resourceRow)
		}

		// Wait for wg
		res.wait()
		// Close channel
		res.close()
	}()

	return res, nil
}

func (rt *ResourceClient) Query(ctx context.Context, filter *spanner.Statement, opts *QueryOptions) ([]*ResourceRow, string, error) {
	messages := []proto.Message{proto.Clone(rt.resourceMsg)}
	if rt.hasIamPolicy {
		messages = append(messages, &iampb.Policy{})
	}
	rows, nextToken, err := rt.tbl.Query(ctx, messages, filter, opts)
	if err != nil {
		return nil, "", err
	}
	resourceRows := make([]*ResourceRow, len(rows))
	for i, row := range rows {
		resourceRow := &ResourceRow{
			Resource:       row.Messages[0],
			resourceClient: rt,
		}
		if rt.hasIamPolicy {
			resourceRow.Policy = row.Messages[1].(*iampb.Policy)
		}
		resourceRows[i] = resourceRow
	}
	return resourceRows, nextToken, nil
}

// Batch update resources. The rows must have the same resource type.
// Pass in names if the rows were retrieved from a list/query operation, meaning the keys are not set.
// No need for names from Read or BatchRead operations.
func (rt *ResourceClient) BatchUpdateResources(ctx context.Context, rows []*ResourceRow, names ...string) error {
	if len(names) != 0 {
		if len(rows) != len(names) {
			return status.Error(codes.InvalidArgument, "names and rows must be of the same length if names are provided")
		}
	}
	tblRows := make([]*Row, len(rows))
	for i, row := range rows {
		key := row.RowKey
		if len(names) != 0 {
			var err error
			key, err = rt.RowKeyConv.GetRowKey(names[i])
			if err != nil {
				return status.Errorf(codes.Internal, "Failed to convert resource name to row key: %v", err)
			}
		}
		tblRows[i] = &Row{
			Key:      spanner.Key{key},
			Messages: []proto.Message{row.Resource},
		}
	}
	return rt.tbl.BatchUpdate(ctx, tblRows)
}

// Batch update policies. The rows must have the same resource type.
// Pass in names if the rows were retrieved from a list/query operation, meaning the keys are not set.
// No need for names from Read or BatchRead operations.
func (rt *ResourceClient) BatchUpdatePolicies(ctx context.Context, rows []*ResourceRow, names ...string) error {
	if len(names) != 0 {
		if len(rows) != len(names) {
			return status.Error(codes.InvalidArgument, "names and rows must be of the same length if names are provided")
		}
	}
	if !rt.hasIamPolicy {
		return status.Error(codes.InvalidArgument, "Policy not allowed because resource type does not have iam policies")
	}
	tblRows := make([]*Row, len(rows))
	for i, row := range rows {
		key := row.RowKey
		if len(names) != 0 {
			var err error
			key, err = rt.RowKeyConv.GetRowKey(names[i])
			if err != nil {
				return status.Errorf(codes.Internal, "Failed to convert resource name to row key: %v", err)
			}
		}
		tblRows[i] = &Row{
			Key:      spanner.Key{key},
			Messages: []proto.Message{row.Policy},
		}
	}
	return rt.tbl.BatchUpdate(ctx, tblRows)
}

func (rt *ResourceClient) BatchDelete(ctx context.Context, names []string) error {
	rowKeys := make([]spanner.Key, len(names))
	for i, name := range names {
		rowKey, err := rt.RowKeyConv.GetRowKey(name)
		if err != nil {
			return status.Errorf(codes.Internal, "Failed to convert resource name to row key: %v", err)
		}
		rowKeys[i] = spanner.Key{rowKey}
	}
	return rt.tbl.BatchDelete(ctx, rowKeys)
}
