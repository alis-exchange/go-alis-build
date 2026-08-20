package protodb_test

import (
	"context"
	"iter"
	"testing"

	protodb "go.alis.build/protodb/v2"
)

// fakeTable is a minimal, zero-dependency implementation of
// protodb.ResourceTable[string] used only to lock the interface's method
// set at compile time. Every method panics — it is never actually called.
type fakeTable struct{}

func (f *fakeTable) Create(ctx context.Context, rows ...*protodb.Row[string]) error {
	panic("not implemented")
}

func (f *fakeTable) Write(ctx context.Context, rows ...*protodb.Row[string]) error {
	panic("not implemented")
}

func (f *fakeTable) Read(ctx context.Context, key protodb.Key) (*protodb.Row[string], error) {
	panic("not implemented")
}

func (f *fakeTable) BatchRead(ctx context.Context, keys ...protodb.Key) ([]*protodb.Row[string], error) {
	panic("not implemented")
}

func (f *fakeTable) List(ctx context.Context, opts protodb.ListOptions) ([]*protodb.Row[string], string, error) {
	panic("not implemented")
}

func (f *fakeTable) Stream(ctx context.Context, opts protodb.StreamOptions) iter.Seq2[*protodb.Row[string], error] {
	panic("not implemented")
}

func (f *fakeTable) Delete(ctx context.Context, keys ...protodb.Key) error {
	panic("not implemented")
}

func (f *fakeTable) WritePolicies(ctx context.Context, entries ...protodb.PolicyEntry) error {
	panic("not implemented")
}

func TestRowIsPureData(t *testing.T) {
	r := &protodb.Row[string]{Key: nil, Resource: "x"}
	if r.Resource != "x" {
		t.Fatal("row literal must be constructible")
	}
	// Interface must be implementable by a zero-dependency fake:
	var _ protodb.ResourceTable[string] = (*fakeTable)(nil)
}
