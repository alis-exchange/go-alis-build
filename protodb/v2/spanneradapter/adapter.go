// Package spanneradapter provides Spanner-specific implementations of the
// protodb package interfaces.
package spanneradapter

import (
	"cloud.google.com/go/spanner"
	"go.alis.build/protodb/v2"
)

// ToKey converts a protodb.Key to a spanner.Key, in the key's column
// order.
func ToKey(k protodb.Key) spanner.Key {
	return spanner.Key(k.KeyValues())
}

// ToKeySets converts keys into a single spanner.KeySet covering all of
// them, for use in a Spanner Read/batch-read call.
func ToKeySets(keys []protodb.Key) spanner.KeySet {
	sets := make([]spanner.KeySet, len(keys))
	for i, k := range keys {
		sets[i] = ToKey(k)
	}
	return spanner.KeySets(sets...)
}
