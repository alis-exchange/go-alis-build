package bqschema

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrDatasetNotFound is returned when the target dataset does not exist.
type ErrDatasetNotFound struct {
	DatasetID string
}

func (e ErrDatasetNotFound) Error() string {
	return fmt.Sprintf("bigquery dataset %q does not exist; create the dataset (e.g. via Terraform or `bq mk`) before starting the reporter", e.DatasetID)
}

func (e ErrDatasetNotFound) Is(target error) bool {
	var err ErrDatasetNotFound
	return errors.As(target, &err)
}

func (e ErrDatasetNotFound) GRPCStatus() *status.Status {
	return status.New(codes.NotFound, e.Error())
}

// ErrDatasetMetadata wraps a failure to read dataset metadata.
type ErrDatasetMetadata struct {
	DatasetID string
	Err       error
}

func (e ErrDatasetMetadata) Error() string {
	return fmt.Sprintf("bigquery dataset %q metadata: %v", e.DatasetID, e.Err)
}

func (e ErrDatasetMetadata) Unwrap() error { return e.Err }

func (e ErrDatasetMetadata) Is(target error) bool {
	var err ErrDatasetMetadata
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrDatasetMetadata) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrTableMetadata wraps a failure to read table metadata.
type ErrTableMetadata struct {
	Qualified string
	Err       error
}

func (e ErrTableMetadata) Error() string {
	return fmt.Sprintf("bigquery table %s metadata: %v", e.Qualified, e.Err)
}

func (e ErrTableMetadata) Unwrap() error { return e.Err }

func (e ErrTableMetadata) Is(target error) bool {
	var err ErrTableMetadata
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrTableMetadata) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrCreateTable wraps a failure to create a BigQuery table.
type ErrCreateTable struct {
	Qualified string
	Err       error
}

func (e ErrCreateTable) Error() string {
	return fmt.Sprintf("create bigquery table %s: %v", e.Qualified, e.Err)
}

func (e ErrCreateTable) Unwrap() error { return e.Err }

func (e ErrCreateTable) Is(target error) bool {
	var err ErrCreateTable
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrCreateTable) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrUpdateTableSchema wraps a failure to additively update a table schema.
type ErrUpdateTableSchema struct {
	Qualified string
	Err       error
}

func (e ErrUpdateTableSchema) Error() string {
	return fmt.Sprintf("update bigquery table %s schema (additive changes only): %v", e.Qualified, e.Err)
}

func (e ErrUpdateTableSchema) Unwrap() error { return e.Err }

func (e ErrUpdateTableSchema) Is(target error) bool {
	var err ErrUpdateTableSchema
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrUpdateTableSchema) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}
