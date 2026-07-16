package bqschema

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/googleapi"
)

func TestEnsureTable_missingDataset(t *testing.T) {
	t.Parallel()
	prov := &fakeProvisioner{
		metaErr: &googleapi.Error{Code: http.StatusNotFound, Message: "dataset not found"},
	}
	err := ensureTableWith(context.Background(), prov, "my-dataset", "my-table")
	if err == nil {
		t.Fatal("expected error for missing dataset")
	}
	for _, want := range []string{"my-dataset", "does not exist", "before starting the reporter"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want to contain %q", err.Error(), want)
		}
	}
	if prov.tableEnsured {
		t.Error("ensure was called despite missing dataset")
	}
}

func TestEnsureTable_createsTable_whenMissing(t *testing.T) {
	t.Parallel()
	prov := &fakeProvisioner{
		tableExists: false,
	}
	if err := ensureTableWith(context.Background(), prov, "ds", "tbl"); err != nil {
		t.Fatalf("EnsureTable: %v", err)
	}
	if !prov.tableCreated {
		t.Fatal("table was not created")
	}
	if prov.createdMD == nil {
		t.Fatal("createdMD is nil")
	}
	if !schemasEqual(prov.createdMD.Schema, Schema()) {
		t.Errorf("created table schema does not equal bqschema.Schema()")
	}
}

func TestEnsureTable_createsTable_forwardsSuppliedMetadata(t *testing.T) {
	t.Parallel()
	prov := &fakeProvisioner{tableExists: false}
	md := bigquery.TableMetadata{
		TimePartitioning: &bigquery.TimePartitioning{Field: "start_time", Type: bigquery.DayPartitioningType},
		Clustering:       &bigquery.Clustering{Fields: []string{"type", "status"}},
		Description:      "Alis Evals — Run rows",
	}
	if err := ensureTableWith(context.Background(), prov, "ds", "tbl", md); err != nil {
		t.Fatalf("EnsureTable: %v", err)
	}
	got := prov.createdMD
	if got.TimePartitioning == nil || got.TimePartitioning.Field != "start_time" {
		t.Errorf("time partitioning not forwarded: %+v", got.TimePartitioning)
	}
	if got.Clustering == nil || len(got.Clustering.Fields) != 2 {
		t.Errorf("clustering not forwarded: %+v", got.Clustering)
	}
	if got.Description != "Alis Evals — Run rows" {
		t.Errorf("description not forwarded: %q", got.Description)
	}
	// Metadata schema is always overwritten with bqschema.Schema().
	if !schemasEqual(got.Schema, Schema()) {
		t.Error("createdMD.Schema was not overwritten with Schema()")
	}
}

func TestEnsureTable_updatesSchema_whenTableExists(t *testing.T) {
	t.Parallel()
	prov := &fakeProvisioner{
		tableExists:    true,
		existingETag:   "etag-42",
		existingSchema: bigquery.Schema{{Name: "old", Type: bigquery.StringFieldType}},
	}
	if err := ensureTableWith(context.Background(), prov, "ds", "tbl"); err != nil {
		t.Fatalf("EnsureTable: %v", err)
	}
	if prov.tableCreated {
		t.Error("existing table should not be recreated")
	}
	if !prov.updateApplied {
		t.Fatal("update was not applied")
	}
	if !schemasEqual(prov.updateSchema, Schema()) {
		t.Error("update schema does not equal bqschema.Schema()")
	}
	if prov.updateETag != "etag-42" {
		t.Errorf("update ETag = %q, want %q", prov.updateETag, "etag-42")
	}
}

func TestEnsureTable_propagatesCreateError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("create boom")
	prov := &fakeProvisioner{tableExists: false, createErr: sentinel}
	err := ensureTableWith(context.Background(), prov, "ds", "tbl")
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want sentinel wrapped", err)
	}
	if !strings.Contains(err.Error(), "ds.tbl") {
		t.Errorf("err = %q, want to contain %q", err.Error(), "ds.tbl")
	}
}

func TestEnsureTable_propagatesUpdateError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("update boom")
	prov := &fakeProvisioner{tableExists: true, updateErr: sentinel}
	err := ensureTableWith(context.Background(), prov, "ds", "tbl")
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want sentinel wrapped", err)
	}
	if !strings.Contains(err.Error(), "ds.tbl") {
		t.Errorf("err = %q, want to contain %q", err.Error(), "ds.tbl")
	}
}

// fakeProvisioner is an in-memory tableProvisioner. It mirrors the shape of
// the production provisioner (dataset check + table create/update) without
// requiring a *bigquery.Client.
type fakeProvisioner struct {
	mu sync.Mutex

	// Test inputs.
	metaErr        error // returned from datasetMetadata
	tableExists    bool
	existingETag   string
	existingSchema bigquery.Schema
	createErr      error
	updateErr      error

	// Recorded outputs.
	tableEnsured  bool
	tableCreated  bool
	createdMD     *bigquery.TableMetadata
	updateApplied bool
	updateSchema  bigquery.Schema
	updateETag    string
}

func (f *fakeProvisioner) datasetMetadata(context.Context) error {
	return f.metaErr
}

func (f *fakeProvisioner) tableMetadata(context.Context) (*bigquery.TableMetadata, error) {
	if !f.tableExists {
		return nil, &googleapi.Error{Code: http.StatusNotFound, Message: "table not found"}
	}
	return &bigquery.TableMetadata{ETag: f.existingETag, Schema: f.existingSchema}, nil
}

func (f *fakeProvisioner) createTable(_ context.Context, md *bigquery.TableMetadata) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tableEnsured = true
	f.tableCreated = true
	if f.createErr != nil {
		return f.createErr
	}
	// Copy to detach from caller mutations.
	copied := *md
	f.createdMD = new(copied)
	return nil
}

func (f *fakeProvisioner) updateSchemaAdditive(_ context.Context, schema bigquery.Schema, etag string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tableEnsured = true
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updateApplied = true
	f.updateSchema = schema
	f.updateETag = etag
	return nil
}

var _ tableProvisioner = (*fakeProvisioner)(nil)
