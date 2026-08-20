package memadapter_test

import (
	"fmt"
	"testing"

	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/memadapter"
	"go.alis.build/protodb/v2/protodbtest"
)

// TestConformance runs the protodbtest conformance suite against
// memadapter, proving memadapter satisfies the documented ResourceTable
// contract. memadapter declares SupportsFilter false: it has no filter
// engine (see memadapter.errFilterUnimplemented), so the suite's
// FilterContract subtest asserts List/Stream return Unimplemented for any
// non-empty Filter.
func TestConformance(t *testing.T) {
	protodbtest.Conformance[string]{
		NewTable: func(t *testing.T) protodb.ResourceTable[string] {
			return memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
		},
		Runner: func(t *testing.T) protodb.TransactionRunner { return memadapter.NewTransactionRunner() },
		MakeRow: func(i int) *protodb.Row[string] {
			return &protodb.Row[string]{Key: strKey(fmt.Sprintf("k%03d", i)), Resource: fmt.Sprintf("v%d", i)}
		},
		Equal: func(a, b string) bool { return a == b },
	}.Run(t)
}
