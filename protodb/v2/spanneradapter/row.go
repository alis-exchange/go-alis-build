package spanneradapter

import (
	"context"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/mennanov/fmutils"
	"go.alis.build/protodb/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// NewSpannerResourceRow constructs a SpannerResourceRow that implements
// database.ResourceRow. The resourceTable is used by Update and Delete to
// persist changes; it participates in transactions when the context
// contains an active Spanner transaction.
func NewSpannerResourceRow[R proto.Message](key protodb.RowKey, resource R, policy *iampb.Policy, resourceTable protodb.ResourceTable[R]) *SpannerResourceRow[R] {
	return &SpannerResourceRow[R]{
		key:           key,
		resource:      resource,
		policy:        policy,
		resourceTable: resourceTable,
	}
}

// SpannerResourceRow is a Spanner-backed implementation of database.ResourceRow.
// It holds the row key, resource proto, and IAM policy, and delegates Update
// and Delete to the underlying ResourceTable.
type SpannerResourceRow[R proto.Message] struct {
	key           protodb.RowKey
	resource      R
	policy        *iampb.Policy
	resourceTable protodb.ResourceTable[R]
}

// Merge updates the resource row with the provided updated message.
//
// It clones the updated message, to avoid modifying the original message.
// It then filters the updated message to only include the specified field mask paths.
// It proceeds to prune the existing resource row's resource using the same field mask paths
// to clear any fields included in the field mask paths.
// Finally, it merges the filtered updated message into the existing resource row's resource.
func (rr *SpannerResourceRow[R]) Merge(updatedMsg proto.Message, fieldMaskPaths ...string) {
	clonedUpdatedMsg := proto.Clone(updatedMsg)
	fmutils.Filter(clonedUpdatedMsg, fieldMaskPaths)
	fmutils.Prune(rr.GetResource(), fieldMaskPaths)
	proto.Merge(rr.GetResource(), clonedUpdatedMsg)
}

// Update updates the resource row in the Spanner database.
func (rr *SpannerResourceRow[R]) Update(ctx context.Context) error {
	if rr.GetRowKey() == nil {
		return status.Error(codes.InvalidArgument, "Row key is empty because row was retrieved via Query,List or Stream method. Use SetRowKey to set the row key")
	}

	if _, err := rr.resourceTable.Write(ctx, rr.GetRowKey(), rr.GetResource(), rr.GetPolicy()); err != nil {
		return err
	}

	return nil
}

// Delete removes the resource row from the Spanner database.
func (rr *SpannerResourceRow[R]) Delete(ctx context.Context) error {
	if rr.GetRowKey() == nil {
		return status.Error(codes.InvalidArgument, "Row key is empty because row was retrieved via Query,List or Stream method. Use SetRowKey to set the row key")
	}

	return rr.resourceTable.Delete(ctx, rr.GetRowKey())
}

// GetRowKey returns the row key of the resource row.
func (rr *SpannerResourceRow[R]) GetRowKey() protodb.RowKey {
	return rr.key
}

// SetRowKey sets the row key of the resource row.
func (rr *SpannerResourceRow[R]) SetRowKey(key protodb.RowKey) {
	rr.key = key
}

// GetResource returns the resource of the resource row.
func (rr *SpannerResourceRow[R]) GetResource() R {
	return rr.resource
}

// SetResource sets the resource of the resource row.
func (rr *SpannerResourceRow[R]) SetResource(resource R) {
	rr.resource = resource
}

// GetPolicy returns the IAM policy associated with the resource row.
func (rr *SpannerResourceRow[R]) GetPolicy() *iampb.Policy {
	return rr.policy
}

// SetPolicy sets the IAM policy associated with the resource row.
func (rr *SpannerResourceRow[R]) SetPolicy(policy *iampb.Policy) {
	rr.policy = policy
}

// ApplyReadMask applies the provided read mask to the resource row's resource,
// filtering out any fields not included in the read mask.
func (rr *SpannerResourceRow[R]) ApplyReadMask(readMask *fieldmaskpb.FieldMask, ignoredPaths ...string) error {
	if readMask == nil {
		return nil
	}

	// Validate the read mask
	if !readMask.IsValid(rr.GetResource()) {
		return status.Errorf(codes.InvalidArgument, "invalid read mask: %v", readMask)
	}

	// If there are ignored paths, add them to the read mask
	// to ensure they are not filtered out.
	if len(ignoredPaths) > 0 {
		for _, path := range ignoredPaths {
			readMask.Paths = append(readMask.GetPaths(), path)
		}
	}

	// Normalize the read mask to remove any duplicates
	readMask.Normalize()

	// Apply the read mask to the resource
	fmutils.Filter(rr.GetResource(), readMask.GetPaths())

	return nil
}
