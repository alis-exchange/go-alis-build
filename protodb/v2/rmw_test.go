package protodb_test

import (
	"context"
	"errors"
	"iter"
	"testing"

	protodb "go.alis.build/protodb/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// rmwKey is a minimal string-valued protodb.Key used only by the
// ReadModifyWrite tests.
type rmwKey string

func (k rmwKey) KeyValues() []any { return []any{string(k)} }

// rmwTable is a minimal, map-backed protodb.ResourceTable[string] used only
// by the ReadModifyWrite tests. Only Read and Write are implemented over the
// map; every other method panics since ReadModifyWrite never calls them.
type rmwTable struct {
	rows map[string]string
}

func (t *rmwTable) Create(ctx context.Context, rows ...*protodb.Row[string]) error {
	panic("not implemented")
}

func (t *rmwTable) Write(ctx context.Context, rows ...*protodb.Row[string]) error {
	for _, r := range rows {
		k := string(r.Key.(rmwKey))
		t.rows[k] = r.Resource
	}
	return nil
}

func (t *rmwTable) Read(ctx context.Context, key protodb.Key) (*protodb.Row[string], error) {
	k := string(key.(rmwKey))
	v, ok := t.rows[k]
	if !ok {
		return nil, status.Error(codes.NotFound, k)
	}
	return &protodb.Row[string]{Key: key, Resource: v}, nil
}

func (t *rmwTable) BatchRead(ctx context.Context, keys ...protodb.Key) ([]*protodb.Row[string], error) {
	panic("not implemented")
}

func (t *rmwTable) List(ctx context.Context, opts protodb.ListOptions) ([]*protodb.Row[string], string, error) {
	panic("not implemented")
}

func (t *rmwTable) Stream(ctx context.Context, opts protodb.StreamOptions) iter.Seq2[*protodb.Row[string], error] {
	panic("not implemented")
}

func (t *rmwTable) Delete(ctx context.Context, keys ...protodb.Key) error {
	panic("not implemented")
}

func (t *rmwTable) WritePolicies(ctx context.Context, entries ...protodb.PolicyEntry) error {
	panic("not implemented")
}

// runnerOnce is a protodb.TransactionRunner that runs fn exactly once and
// returns its error directly — enough to exercise ReadModifyWrite without a
// real transactional backend.
type runnerOnce struct{}

func (runnerOnce) RunTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// runnerTwice models a Spanner abort/retry: it runs fn twice, discarding the
// first attempt's error, and returns the result of the second. It exists to
// prove fn's re-runs are safe when fn only mutates the row it is given.
type runnerTwice struct{ runs int }

func (r *runnerTwice) RunTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	r.runs++
	_ = fn(ctx)
	r.runs++
	return fn(ctx)
}

func TestReadModifyWrite(t *testing.T) {
	tbl := &rmwTable{rows: map[string]string{"k": "v0"}}
	var calls int
	err := protodb.ReadModifyWrite(context.Background(), runnerOnce{}, tbl, rmwKey("k"),
		func(r *protodb.Row[string]) error { calls++; r.Resource = "v1"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if tbl.rows["k"] != "v1" {
		t.Fatal("write not applied")
	}
	if calls != 1 {
		t.Fatalf("want 1 call, got %d", calls)
	}
}

func TestReadModifyWriteNotFoundPropagates(t *testing.T) {
	tbl := &rmwTable{rows: map[string]string{}}
	var called bool
	err := protodb.ReadModifyWrite(context.Background(), runnerOnce{}, tbl, rmwKey("missing"),
		func(r *protodb.Row[string]) error { called = true; return nil })
	if !protodb.IsNotFound(err) {
		t.Fatalf("want NotFound, got %v", err)
	}
	if called {
		t.Fatal("fn must not be called when Read fails")
	}
}

func TestReadModifyWriteFnErrorAborts(t *testing.T) {
	tbl := &rmwTable{rows: map[string]string{"k": "v0"}}
	sentinel := errors.New("nope")
	err := protodb.ReadModifyWrite(context.Background(), runnerOnce{}, tbl, rmwKey("k"),
		func(r *protodb.Row[string]) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if tbl.rows["k"] != "v0" {
		t.Fatal("must not write after fn error")
	}
}

// TestReadModifyWriteSurvivesRetry proves fn's mutation is safe to re-run:
// a runner that retries the transaction (as Spanner does on abort) still
// converges on the correct final state, since fn only mutates the row it is
// given each time.
func TestReadModifyWriteSurvivesRetry(t *testing.T) {
	tbl := &rmwTable{rows: map[string]string{"k": "v0"}}
	var calls int
	runner := &runnerTwice{}
	err := protodb.ReadModifyWrite(context.Background(), runner, tbl, rmwKey("k"),
		func(r *protodb.Row[string]) error { calls++; r.Resource = "v1"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if tbl.rows["k"] != "v1" {
		t.Fatal("write not applied")
	}
	if calls != 2 {
		t.Fatalf("want fn to run twice under retry, got %d", calls)
	}
}
