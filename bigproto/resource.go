package bigproto

import (
	"context"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/mennanov/fmutils"
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
	// The name of the column family that contains the resource. If not provided, 'r' is used.
	ResourceColumnFamily string
	// The name of the column family that contains the policy. If not provided, 'p' is used.
	PolicyColumnFamily string
}

type ResourceClient struct {
	tbl                               *BigProto
	RowKeyConv                        *RowKeyConverter
	resourceMsg                       proto.Message
	hasIamPolicy                      bool
	returnPermissionDeniedForNotFound bool
	resourceColumnFamily              string
	policyColumnFamily                string
	defaultListLimit                  int32
}

type ResourceRow struct {
	RowKey         string
	Resource       proto.Message
	Policy         *iampb.Policy
	resourceClient *ResourceClient
}

// Update the resource in the database. Does not affect the policy.
func (rr *ResourceRow) Update(ctx context.Context) error {
	return rr.resourceClient.tbl.WriteProto(ctx, rr.RowKey, rr.resourceClient.resourceColumnFamily, rr.Resource)
}

func (rr *ResourceRow) Delete(ctx context.Context) error {
	return rr.resourceClient.tbl.DeleteRow(ctx, rr.RowKey)
}

func (rr *ResourceRow) Merge(updatedMsg proto.Message, fieldMaskPaths ...string) {
	fmutils.Filter(updatedMsg, fieldMaskPaths)
	fmutils.Prune(rr.Resource, fieldMaskPaths)
	proto.Merge(rr.Resource, updatedMsg)
}

func (d *BigProto) NewResourceClient(prefix string, msg proto.Message, options *ResourceTblOptions) *ResourceClient {
	if options == nil {
		options = &ResourceTblOptions{}
	}
	if options.DefaultLimit == 0 {
		options.DefaultLimit = 100
	}
	if options.ResourceColumnFamily == "" {
		options.ResourceColumnFamily = "r"
	}
	if options.PolicyColumnFamily == "" {
		options.PolicyColumnFamily = "p"
	}

	rt := &ResourceClient{
		tbl:                               d,
		resourceMsg:                       msg,
		RowKeyConv:                        &RowKeyConverter{AbbreviateCollectionIdentifiers: true, LatestVersionFirst: options.IsVersion, KeyPrefix: prefix},
		hasIamPolicy:                      options.HasIamPolicy,
		returnPermissionDeniedForNotFound: options.ReturnPermissionDeniedForNotFound,
		resourceColumnFamily:              options.ResourceColumnFamily,
		policyColumnFamily:                options.PolicyColumnFamily,
		defaultListLimit:                  int32(options.DefaultLimit),
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
	colFamilies := []string{rt.resourceColumnFamily}
	if rt.hasIamPolicy {
		if policy.Etag == nil {
			return nil, status.Error(codes.InvalidArgument, "Policy etag is required")
		}
		msgs = append(msgs, policy)
		colFamilies = append(colFamilies, rt.policyColumnFamily)
	}
	err = rt.tbl.WriteProtos(ctx, rowKey, colFamilies, msgs)
	if err != nil {
		return nil, err
	}
	resourceRow := &ResourceRow{
		RowKey:         rowKey,
		Resource:       resource,
		resourceClient: rt,
	}
	if rt.hasIamPolicy {
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
	policy := &iampb.Policy{}

	if rt.hasIamPolicy {
		policy, err = rt.tbl.ReadProtoWithPolicy(ctx, rowKey, rt.resourceColumnFamily, msg, &fieldmaskpb.FieldMask{Paths: fieldMaskPaths}, rt.policyColumnFamily)
	} else {
		err = rt.tbl.ReadProto(ctx, rowKey, rt.resourceColumnFamily, msg, &fieldmaskpb.FieldMask{Paths: fieldMaskPaths})
	}
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
	return rt.tbl.WriteProto(ctx, rowKey, rt.policyColumnFamily, policy)
}

type ListOptions struct {
	PageSize  int32
	NextToken string
	ReadMask  *fieldmaskpb.FieldMask
}

func (rt *ResourceClient) List(ctx context.Context, parent string, opts *ListOptions) ([]*ResourceRow, string, error) {
	msg := proto.Clone(rt.resourceMsg)
	prefix, err := rt.RowKeyConv.GetRowKeyPrefix(parent)
	if err != nil {
		return nil, "", status.Errorf(codes.Internal, "Failed to convert resource name to row key: %v", err)
	}
	policyColFamily := ""
	if rt.hasIamPolicy {
		policyColFamily = rt.policyColumnFamily
	}
	rowsWithPolicies, nextToken, err := rt.tbl.PageProtosWithPolicies(ctx, rt.resourceColumnFamily, msg, policyColFamily, PageOptions{
		RowKeyPrefix: prefix,
		PageSize:     opts.PageSize,
		NextToken:    opts.NextToken,
		ReadMask:     opts.ReadMask,
		MaxPageSize:  rt.defaultListLimit,
	})
	if err != nil {
		return nil, "", err
	}
	resourceRows := make([]*ResourceRow, len(rowsWithPolicies))
	for i, row := range rowsWithPolicies {
		resourceRows[i] = &ResourceRow{
			RowKey:         row.Key,
			Resource:       row.Row,
			Policy:         row.Policy,
			resourceClient: rt,
		}
	}
	return resourceRows, nextToken, nil
}
