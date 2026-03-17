// Package spanneradapter provides Spanner-specific implementations of the
// protodb package interfaces: SpannerRowKeyFactory for key decoding,
// SpannerTransactionRunner for transactional execution, NewSpannerBaseResourceRow
// for BaseResourceRow construction, and NewSpannerResourceRow for ResourceRow construction.
package spanneradapter

import (
	"cloud.google.com/go/spanner"
	"go.alis.build/protodb/v2"
)

// SpannerRowKeyFactory extends the core RowKeyFactory with Spanner-specific
// row scanning. Each table implementation depends on this rather than the
// core interface, keeping Spanner concerns in the adapter layer.
type SpannerRowKeyFactory interface {
	protodb.RowKeyFactory
	// Decode extracts a RowKey from a Spanner row result.
	Decode(row *spanner.Row) (protodb.RowKey, error)
}

// ToKey converts a database-agnostic RowKey to a spanner.Key for use in
// ReadRow, Read, and Delete operations.
func ToKey(k protodb.RowKey) spanner.Key {
	return spanner.Key(k.KeyValues())
}

// ToKeys converts a slice of RowKeys to spanner.KeySets for batch Read
// and Delete operations.
func ToKeys(rowKeys []protodb.RowKey) []spanner.KeySet {
	keySets := make([]spanner.KeySet, len(rowKeys))
	for i, k := range rowKeys {
		key := ToKey(k)
		keySets[i] = key
	}
	return keySets
}
