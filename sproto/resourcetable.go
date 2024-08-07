package sproto

import (
	"context"
	"fmt"

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
}

type ResourceClient struct {
	tbl                               *TableClient
	RowKeyConv                        *RowKeyConverter
	resourceMsg                       proto.Message
	hasIamPolicy                      bool
	returnPermissionDeniedForNotFound bool
	keyColumnName                     string
}

type ResourceRow struct {
	RowKey   string
	Resource proto.Message
	Policy   *iampb.Policy
	tbl      *TableClient
}

func (rr *ResourceRow) Merge(updatedMsg proto.Message, fieldMaskPaths ...string) {
	fmutils.Filter(updatedMsg, fieldMaskPaths)
	fmutils.Prune(rr.Resource, fieldMaskPaths)
	proto.Merge(rr.Resource, updatedMsg)
}

// Update the resource in the database. Does not update the policy.
func (rr *ResourceRow) Update(ctx context.Context) error {
	return rr.tbl.Update(ctx, spanner.Key{rr.RowKey}, rr.Resource)
}

func (rr *ResourceRow) Delete(ctx context.Context) error {
	return rr.tbl.Delete(ctx, spanner.Key{rr.RowKey})
}

func (d *DbClient) NewResourceClient(tableName string, msg proto.Message, options *ResourceTblOptions) *ResourceClient {
	if options == nil {
		options = &ResourceTblOptions{}
	}
	if options.DefaultLimit == 0 {
		options.DefaultLimit = 100
	}
	if options.KeyColumnName == "" {
		options.KeyColumnName = "key"
	}
	tableClient, err := d.NewTableClient(tableName, options.DefaultLimit)
	if err != nil {
		alog.Fatalf(context.Background(), "Failed to create table client: %v", err)
	}
	rt := &ResourceClient{
		tbl:                               tableClient,
		resourceMsg:                       msg,
		RowKeyConv:                        &RowKeyConverter{AbbreviateCollectionIdentifiers: true, LatestVersionFirst: options.IsVersion},
		hasIamPolicy:                      options.HasIamPolicy,
		returnPermissionDeniedForNotFound: options.ReturnPermissionDeniedForNotFound,
	}
	return rt
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
		RowKey:   rowKey,
		Resource: resource,
	}
	if rt.hasIamPolicy {
		if policy.Etag == nil {
			return nil, status.Error(codes.InvalidArgument, "Policy etag is required")
		}
		resourceRow.Policy = policy
	}
	return resourceRow, nil
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
		if spanner.ErrCode(err) == codes.NotFound && rt.returnPermissionDeniedForNotFound {
			return nil, status.Errorf(codes.PermissionDenied, "you do not have the required permission to access this resource")
		}
		return nil, err
	}
	resourceRow := &ResourceRow{
		RowKey:   rowKey,
		Resource: msg,
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

func (rt *ResourceClient) BatchRead(ctx context.Context, names []string, fieldMaskPaths ...string) ([]*ResourceRow, error) {
	rowKeys := make([]spanner.Key, len(names))
	for i, name := range names {
		rowKey, err := rt.RowKeyConv.GetRowKey(name)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to convert resource name to row key: %v", err)
		}
		rowKeys[i] = spanner.Key{rowKey}
	}
	msgs := []proto.Message{proto.Clone(rt.resourceMsg)}
	if rt.hasIamPolicy {
		msgs = append(msgs, &iampb.Policy{})
	}
	rows, err := rt.tbl.BatchReadWithFieldMask(ctx, rowKeys, msgs, []*fieldmaskpb.FieldMask{{Paths: fieldMaskPaths}})
	if err != nil {
		if spanner.ErrCode(err) == codes.NotFound && rt.returnPermissionDeniedForNotFound {
			return nil, status.Errorf(codes.PermissionDenied, "you do not have the required permission to access this resource")
		}
		return nil, err
	}
	resourceRows := make([]*ResourceRow, len(names))
	for i, row := range rows {
		resourceRow := &ResourceRow{
			RowKey:   rowKeys[i].String(),
			Resource: row.Messages[0],
		}
		if rt.hasIamPolicy {
			resourceRow.Policy = row.Messages[1].(*iampb.Policy)
		}
		resourceRows[i] = resourceRow
	}
	return resourceRows, nil
}

func (rt *ResourceClient) List(ctx context.Context, parent string, opts *QueryOptions) ([]*ResourceRow, string, error) {
	var err error
	spannerStatement := spanner.NewStatement(fmt.Sprintf("STARTS_WITH(%s,@prefix)", rt.keyColumnName))
	spannerStatement.Params["prefix"], err = rt.RowKeyConv.GetRowKeyPrefix(parent)
	if err != nil {
		return nil, "", status.Errorf(codes.Internal, "Failed to convert parent name to row key prefix: %v", err)
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
			RowKey:   row.Key.String(),
			Resource: row.Messages[0],
		}
		if rt.hasIamPolicy {
			resourceRow.Policy = row.Messages[1].(*iampb.Policy)
		}
		resourceRows[i] = resourceRow
	}
	return resourceRows, nextToken, nil
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
			RowKey:   row.Key.String(),
			Resource: row.Messages[0],
		}
		if rt.hasIamPolicy {
			resourceRow.Policy = row.Messages[1].(*iampb.Policy)
		}
		resourceRows[i] = resourceRow
	}
	return resourceRows, nextToken, nil
}
