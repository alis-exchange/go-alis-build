package spanneradapter

import (
	"context"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner"
	"github.com/mennanov/fmutils"
	"go.alis.build/protodb/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// SpannerRowKeyFactory extends the core RowKeyFactory with Spanner-specific
// row scanning. Each table implementation depends on this rather than the
// core interface, keeping Spanner concerns in the adapter layer.
type SpannerRowKeyFactory interface {
	protodb.RowKeyFactory
	// Decode extracts a RowKey from a Spanner row result.
	Decode(row *spanner.Row) (protodb.RowKey, error)
}

// ToKey converts a database-agnostic RowKey to a spanner.Key.
func ToKey(k protodb.RowKey) spanner.Key {
	return spanner.Key(k.KeyValues())
}

// ToKeys converts a slice of RowKeys to spanner.KeySets.
func ToKeys(rowKeys []protodb.RowKey) []spanner.KeySet {
	keySets := make([]spanner.KeySet, len(rowKeys))
	for i, k := range rowKeys {
		key := ToKey(k)
		keySets[i] = key
	}
	return keySets
}

func NewSpannerResourceRow[R proto.Message](key protodb.RowKey, resource R, policy *iampb.Policy, resourceTable protodb.ResourceTable[R]) *SpannerResourceRow[R] {
	return &SpannerResourceRow[R]{
		key:           key,
		resource:      resource,
		policy:        policy,
		resourceTable: resourceTable,
	}
}

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
