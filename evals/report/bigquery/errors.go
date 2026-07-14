package bigquery

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNewClient wraps a failure to construct a BigQuery client.
type ErrNewClient struct {
	Err error
}

func (e ErrNewClient) Error() string {
	return fmt.Sprintf("bigquery.NewClient: %v", e.Err)
}

func (e ErrNewClient) Unwrap() error { return e.Err }

func (e ErrNewClient) Is(target error) bool {
	var err ErrNewClient
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrNewClient) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrNilClient is returned when NewWithClient is called with a nil client.
type ErrNilClient struct{}

func (e ErrNilClient) Error() string { return "bigquery client is nil" }

func (e ErrNilClient) Is(target error) bool {
	var err ErrNilClient
	return errors.As(target, &err)
}

func (e ErrNilClient) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrEmptyProjectID is returned when the BigQuery project ID is empty.
type ErrEmptyProjectID struct{}

func (e ErrEmptyProjectID) Error() string { return "bigquery project ID is empty" }

func (e ErrEmptyProjectID) Is(target error) bool {
	var err ErrEmptyProjectID
	return errors.As(target, &err)
}

func (e ErrEmptyProjectID) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrEmptyDatasetID is returned when the BigQuery dataset ID is empty.
type ErrEmptyDatasetID struct{}

func (e ErrEmptyDatasetID) Error() string { return "bigquery dataset ID is empty" }

func (e ErrEmptyDatasetID) Is(target error) bool {
	var err ErrEmptyDatasetID
	return errors.As(target, &err)
}

func (e ErrEmptyDatasetID) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrEmptyTableID is returned when the BigQuery table ID is empty.
type ErrEmptyTableID struct{}

func (e ErrEmptyTableID) Error() string { return "bigquery table ID is empty" }

func (e ErrEmptyTableID) Is(target error) bool {
	var err ErrEmptyTableID
	return errors.As(target, &err)
}

func (e ErrEmptyTableID) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInsert wraps a failure to insert a row into BigQuery.
type ErrInsert struct {
	DatasetID string
	TableID   string
	Err       error
}

func (e ErrInsert) Error() string {
	return fmt.Sprintf("bigquery insert into %s.%s: %v", e.DatasetID, e.TableID, e.Err)
}

func (e ErrInsert) Unwrap() error { return e.Err }

func (e ErrInsert) Is(target error) bool {
	var err ErrInsert
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrInsert) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}
