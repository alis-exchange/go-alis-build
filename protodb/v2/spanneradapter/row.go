package spanneradapter

import (
	"context"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/protodb/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewSpannerBaseResourceRow constructs a SpannerBaseResourceRow that implements
// BaseResourceRow[R]. The resourceTable is used by Update and Delete to persist
// changes; it participates in transactions when the context contains an active
// Spanner transaction.
func NewSpannerBaseResourceRow[R any](key protodb.RowKey, resource R, policy *iampb.Policy, resourceTable protodb.BaseResourceTable[R]) *SpannerBaseResourceRow[R] {
	return &SpannerBaseResourceRow[R]{
		key:           key,
		resource:      resource,
		policy:        policy,
		resourceTable: resourceTable,
	}
}

// SpannerBaseResourceRow is a Spanner-backed implementation of BaseResourceRow[R].
// It holds the row key, resource, and IAM policy, and delegates Update and Delete
// to the underlying BaseResourceTable[R].
type SpannerBaseResourceRow[R any] struct {
	key           protodb.RowKey
	resource      R
	policy        *iampb.Policy
	resourceTable protodb.BaseResourceTable[R]
}

// Update updates the resource row in the Spanner database.
func (rr *SpannerBaseResourceRow[R]) Update(ctx context.Context) error {
	if rr == nil {
		return status.Error(codes.InvalidArgument, "Resource row is nil")
	}

	if rr.GetRowKey() == nil {
		return status.Error(codes.InvalidArgument, "Row key is empty because row was retrieved via Query,List or Stream method. Use SetRowKey to set the row key")
	}

	if _, err := rr.resourceTable.Write(ctx, rr.GetRowKey(), rr.GetResource(), rr.GetPolicy()); err != nil {
		return err
	}

	return nil
}

// Delete removes the resource row from the Spanner database.
func (rr *SpannerBaseResourceRow[R]) Delete(ctx context.Context) error {
	if rr == nil {
		return status.Error(codes.InvalidArgument, "Resource row is nil")
	}

	if rr.GetRowKey() == nil {
		return status.Error(codes.InvalidArgument, "Row key is empty because row was retrieved via Query,List or Stream method. Use SetRowKey to set the row key")
	}

	return rr.resourceTable.Delete(ctx, rr.GetRowKey())
}

// GetRowKey returns the row key of the resource row.
func (rr *SpannerBaseResourceRow[R]) GetRowKey() protodb.RowKey {
	if rr == nil {
		return nil
	}

	return rr.key
}

// SetRowKey sets the row key of the resource row.
func (rr *SpannerBaseResourceRow[R]) SetRowKey(key protodb.RowKey) {
	if rr == nil {
		return
	}

	rr.key = key
}

// GetResource returns the resource of the resource row.
func (rr *SpannerBaseResourceRow[R]) GetResource() R {
	if rr == nil {
		var zeroValue R
		return zeroValue
	}

	return rr.resource
}

// SetResource sets the resource of the resource row.
func (rr *SpannerBaseResourceRow[R]) SetResource(resource R) {
	if rr == nil {
		return
	}

	rr.resource = resource
}

// GetPolicy returns the IAM policy associated with the resource row.
func (rr *SpannerBaseResourceRow[R]) GetPolicy() *iampb.Policy {
	if rr == nil {
		return nil
	}

	return rr.policy
}

// SetPolicy sets the IAM policy associated with the resource row.
func (rr *SpannerBaseResourceRow[R]) SetPolicy(policy *iampb.Policy) {
	if rr == nil {
		return
	}

	rr.policy = policy
}
