package pubsub

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrEmptyProjectID is returned when New cannot resolve a Google Cloud project ID.
type ErrEmptyProjectID struct {
	EnvVar string
}

func (e ErrEmptyProjectID) Error() string {
	return fmt.Sprintf("pubsub reporter: project ID is empty (set %s or pass WithProject)", e.EnvVar)
}

func (e ErrEmptyProjectID) Is(target error) bool {
	var err ErrEmptyProjectID
	return errors.As(target, &err)
}

func (e ErrEmptyProjectID) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNewClient wraps a failure to construct a Pub/Sub client.
type ErrNewClient struct {
	Err error
}

func (e ErrNewClient) Error() string {
	return fmt.Sprintf("pubsub.NewClient: %v", e.Err)
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

func (e ErrNilClient) Error() string { return "pubsub client is nil" }

func (e ErrNilClient) Is(target error) bool {
	var err ErrNilClient
	return errors.As(target, &err)
}

func (e ErrNilClient) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrWithProjectWithClient is returned when WithProject is passed to NewWithClient.
type ErrWithProjectWithClient struct{}

func (e ErrWithProjectWithClient) Error() string {
	return "pubsub reporter: WithProject is not valid with NewWithClient (the client already has a project)"
}

func (e ErrWithProjectWithClient) Is(target error) bool {
	var err ErrWithProjectWithClient
	return errors.As(target, &err)
}

func (e ErrWithProjectWithClient) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrPublishMarshal wraps a failure to marshal a run for Pub/Sub publish.
type ErrPublishMarshal struct {
	Topic string
	Err   error
}

func (e ErrPublishMarshal) Error() string {
	return fmt.Sprintf("pubsub publish %s: protojson marshal: %v", e.Topic, e.Err)
}

func (e ErrPublishMarshal) Unwrap() error { return e.Err }

func (e ErrPublishMarshal) Is(target error) bool {
	var err ErrPublishMarshal
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrPublishMarshal) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrPublish wraps a failure to publish a message to Pub/Sub.
type ErrPublish struct {
	Topic string
	Err   error
}

func (e ErrPublish) Error() string {
	return fmt.Sprintf("pubsub publish %s: %v", e.Topic, e.Err)
}

func (e ErrPublish) Unwrap() error { return e.Err }

func (e ErrPublish) Is(target error) bool {
	var err ErrPublish
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrPublish) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}
