package bqschema

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"cloud.google.com/go/bigquery"
	"go.alis.build/alog"
	"google.golang.org/api/googleapi"
)

// tableProvisioner is the internal provisioning seam. It exposes narrow
// operations the production implementation forwards to the BigQuery API and
// that tests substitute with an in-memory fake.
type tableProvisioner interface {
	// datasetMetadata verifies the target dataset exists.
	datasetMetadata(ctx context.Context) error
	// tableMetadata reads the current table metadata, or returns a not-found error.
	tableMetadata(ctx context.Context) (*bigquery.TableMetadata, error)
	// createTable creates the table with the supplied metadata (schema already set).
	createTable(ctx context.Context, md *bigquery.TableMetadata) error
	// updateSchemaAdditive applies an etag-guarded additive schema update.
	updateSchemaAdditive(ctx context.Context, schema bigquery.Schema, etag string) error
}

// EnsureTable checks whether the target table exists in the given dataset
// and creates or additively-updates it to match [Schema]. The dataset must
// already exist; a missing dataset returns an actionable error that names
// it. The client must be non-nil; nil-safety is the caller's
// responsibility.
//
// The optional TableMetadata is applied only when creating the table.
// Existing tables keep their metadata (partitioning, clustering,
// expiration, labels), so post-hoc changes to those must be made outside
// EnsureTable. The metadata's Schema field is always overwritten with
// [Schema].
func EnsureTable(
	ctx context.Context,
	client *bigquery.Client,
	datasetID, tableID string,
	md ...bigquery.TableMetadata,
) error {
	prov := &bqTableProvisioner{
		dataset:      client.Dataset(datasetID),
		datasetLabel: fmt.Sprintf("%s:%s", client.Project(), datasetID),
		tableID:      tableID,
	}
	return ensureTableWith(ctx, prov, datasetID, tableID, md...)
}

// ensureTableWith is a test seam that runs the same provisioning logic as
// [EnsureTable] with an injected provisioner. datasetID and tableID are
// used only for error messages and log lines — the provisioner already
// knows which table it's targeting.
func ensureTableWith(ctx context.Context, prov tableProvisioner, datasetID, tableID string, md ...bigquery.TableMetadata) error {
	var tableMD *bigquery.TableMetadata
	if len(md) > 0 {
		m := md[0]
		tableMD = new(m)
	}
	schema := Schema()
	qualified := datasetID + "." + tableID

	if err := prov.datasetMetadata(ctx); err != nil {
		if isNotFound(err) {
			return ErrDatasetNotFound{DatasetID: datasetID}
		}
		return ErrDatasetMetadata{DatasetID: datasetID, Err: err}
	}

	meta, err := prov.tableMetadata(ctx)
	if err != nil {
		if !isNotFound(err) {
			return ErrTableMetadata{Qualified: qualified, Err: err}
		}
		create := &bigquery.TableMetadata{}
		if tableMD != nil {
			*create = *tableMD
		}
		create.Schema = schema
		if err := prov.createTable(ctx, create); err != nil {
			return ErrCreateTable{Qualified: qualified, Err: err}
		}
		alog.Infof(ctx, "bqschema: created table %s", qualified)
		return nil
	}

	if err := prov.updateSchemaAdditive(ctx, schema, meta.ETag); err != nil {
		return ErrUpdateTableSchema{Qualified: qualified, Err: err}
	}
	alog.Debugf(ctx, "bqschema: ensured schema for table %s", qualified)
	return nil
}

// bqTableProvisioner is the production [tableProvisioner]. It forwards
// operations to the BigQuery Dataset and Table APIs.
type bqTableProvisioner struct {
	// dataset is the target dataset handle.
	dataset *bigquery.Dataset
	// datasetLabel is "project:dataset" for error messages.
	datasetLabel string
	// tableID is the bare table ID within dataset.
	tableID string
}

// datasetMetadata implements tableProvisioner.
func (p *bqTableProvisioner) datasetMetadata(ctx context.Context) error {
	_, err := p.dataset.Metadata(ctx)
	return err
}

// tableMetadata implements tableProvisioner.
func (p *bqTableProvisioner) tableMetadata(ctx context.Context) (*bigquery.TableMetadata, error) {
	return p.dataset.Table(p.tableID).Metadata(ctx)
}

// createTable implements tableProvisioner.
func (p *bqTableProvisioner) createTable(ctx context.Context, md *bigquery.TableMetadata) error {
	return p.dataset.Table(p.tableID).Create(ctx, md)
}

// updateSchemaAdditive implements tableProvisioner.
func (p *bqTableProvisioner) updateSchemaAdditive(ctx context.Context, schema bigquery.Schema, etag string) error {
	update := bigquery.TableMetadataToUpdate{Schema: schema}
	_, err := p.dataset.Table(p.tableID).Update(ctx, update, etag)
	return err
}

// isNotFound reports whether err is a BigQuery API 404 (dataset or table missing).
func isNotFound(err error) bool {
	var gerr *googleapi.Error
	return errors.As(err, &gerr) && gerr.Code == http.StatusNotFound
}
