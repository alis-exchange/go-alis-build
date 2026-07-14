package adk

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrRunEvalFailed indicates the launcher returned a non-success HTTP status.
type ErrRunEvalFailed struct {
	StatusCode int
	Body       string
}

func (e ErrRunEvalFailed) Error() string {
	return fmt.Sprintf("adk: run eval failed: status %d: %s", e.StatusCode, e.Body)
}

func (e ErrRunEvalFailed) Is(target error) bool {
	var err ErrRunEvalFailed
	return errors.As(target, &err)
}

func (e ErrRunEvalFailed) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrAgentUnreachable indicates the HTTP request could not complete.
type ErrAgentUnreachable struct {
	Cause error
}

func (e ErrAgentUnreachable) Error() string {
	if e.Cause == nil {
		return "adk: agent unreachable"
	}
	return fmt.Sprintf("adk: agent unreachable: %v", e.Cause)
}

func (e ErrAgentUnreachable) Unwrap() error { return e.Cause }

func (e ErrAgentUnreachable) Is(target error) bool {
	var err ErrAgentUnreachable
	return errors.As(target, &err) || errors.Is(e.Cause, target)
}

func (e ErrAgentUnreachable) GRPCStatus() *status.Status {
	return status.New(codes.Unavailable, e.Error())
}

// ErrNilClient is returned when an ADK HTTP client method is invoked on a
// nil receiver.
type ErrNilClient struct{}

func (e ErrNilClient) Error() string { return "adk client: nil client" }

func (e ErrNilClient) Is(target error) bool {
	var err ErrNilClient
	return errors.As(target, &err)
}

func (e ErrNilClient) GRPCStatus() *status.Status {
	return status.New(codes.FailedPrecondition, e.Error())
}

// ErrMissingAppNameEvalSetID is returned when RunEval is called without an
// app name or eval set id.
type ErrMissingAppNameEvalSetID struct{}

func (e ErrMissingAppNameEvalSetID) Error() string {
	return "adk client: app name and eval set id are required"
}

func (e ErrMissingAppNameEvalSetID) Is(target error) bool {
	var err ErrMissingAppNameEvalSetID
	return errors.As(target, &err)
}

func (e ErrMissingAppNameEvalSetID) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrMissingAppName is returned when ListEvalSets is called without an app name.
type ErrMissingAppName struct{}

func (e ErrMissingAppName) Error() string { return "adk client: app name is required" }

func (e ErrMissingAppName) Is(target error) bool {
	var err ErrMissingAppName
	return errors.As(target, &err)
}

func (e ErrMissingAppName) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrNilProvider is returned when Provider.Run is called on a nil receiver.
type ErrNilProvider struct{}

func (e ErrNilProvider) Error() string { return "adk provider: nil provider" }

func (e ErrNilProvider) Is(target error) bool {
	var err ErrNilProvider
	return errors.As(target, &err)
}

func (e ErrNilProvider) GRPCStatus() *status.Status {
	return status.New(codes.FailedPrecondition, e.Error())
}

// ErrMissingProviderConfig is returned when Provider.Run is called without
// a base URL or app name.
type ErrMissingProviderConfig struct{}

func (e ErrMissingProviderConfig) Error() string {
	return "adk provider: base URL and app name are required"
}

func (e ErrMissingProviderConfig) Is(target error) bool {
	var err ErrMissingProviderConfig
	return errors.As(target, &err)
}

func (e ErrMissingProviderConfig) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrEmptyBaseURL is returned when a base URL is empty after normalisation.
type ErrEmptyBaseURL struct{}

func (e ErrEmptyBaseURL) Error() string { return "adk: empty base URL" }

func (e ErrEmptyBaseURL) Is(target error) bool {
	var err ErrEmptyBaseURL
	return errors.As(target, &err)
}

func (e ErrEmptyBaseURL) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrInvalidBaseURL is returned when a base URL cannot yield a host.
type ErrInvalidBaseURL struct {
	URL string
}

func (e ErrInvalidBaseURL) Error() string {
	return fmt.Sprintf("adk: invalid base URL %q", e.URL)
}

func (e ErrInvalidBaseURL) Is(target error) bool {
	var err ErrInvalidBaseURL
	return errors.As(target, &err)
}

func (e ErrInvalidBaseURL) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Error())
}

// ErrEncodeRequest wraps a failure to encode an ADK HTTP request body.
type ErrEncodeRequest struct {
	Err error
}

func (e ErrEncodeRequest) Error() string {
	return fmt.Sprintf("adk client: encode request: %v", e.Err)
}

func (e ErrEncodeRequest) Unwrap() error { return e.Err }

func (e ErrEncodeRequest) Is(target error) bool {
	var err ErrEncodeRequest
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrEncodeRequest) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrBuildRequest wraps a failure to construct an ADK HTTP request.
type ErrBuildRequest struct {
	Err error
}

func (e ErrBuildRequest) Error() string {
	return fmt.Sprintf("adk client: build request: %v", e.Err)
}

func (e ErrBuildRequest) Unwrap() error { return e.Err }

func (e ErrBuildRequest) Is(target error) bool {
	var err ErrBuildRequest
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrBuildRequest) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrReadResponse wraps a failure to read an ADK HTTP response body.
type ErrReadResponse struct {
	Err error
}

func (e ErrReadResponse) Error() string {
	return fmt.Sprintf("adk client: read response: %v", e.Err)
}

func (e ErrReadResponse) Unwrap() error { return e.Err }

func (e ErrReadResponse) Is(target error) bool {
	var err ErrReadResponse
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrReadResponse) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrDecodeResponse wraps a failure to decode an ADK run_eval response.
type ErrDecodeResponse struct {
	Err error
}

func (e ErrDecodeResponse) Error() string {
	return fmt.Sprintf("adk client: decode response: %v", e.Err)
}

func (e ErrDecodeResponse) Unwrap() error { return e.Err }

func (e ErrDecodeResponse) Is(target error) bool {
	var err ErrDecodeResponse
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrDecodeResponse) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrDecodeEvalSets wraps a failure to decode an ADK eval set list response.
type ErrDecodeEvalSets struct {
	Err error
}

func (e ErrDecodeEvalSets) Error() string {
	return fmt.Sprintf("adk client: decode eval sets: %v", e.Err)
}

func (e ErrDecodeEvalSets) Unwrap() error { return e.Err }

func (e ErrDecodeEvalSets) Is(target error) bool {
	var err ErrDecodeEvalSets
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrDecodeEvalSets) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrEncodeMetric wraps a failure to encode an eval metric for the ADK API.
type ErrEncodeMetric struct {
	Err error
}

func (e ErrEncodeMetric) Error() string {
	return fmt.Sprintf("adk: encode metric: %v", e.Err)
}

func (e ErrEncodeMetric) Unwrap() error { return e.Err }

func (e ErrEncodeMetric) Is(target error) bool {
	var err ErrEncodeMetric
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrEncodeMetric) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrDecodeMetric wraps a failure to decode an eval metric from the ADK API.
type ErrDecodeMetric struct {
	Err error
}

func (e ErrDecodeMetric) Error() string {
	return fmt.Sprintf("adk: decode metric: %v", e.Err)
}

func (e ErrDecodeMetric) Unwrap() error { return e.Err }

func (e ErrDecodeMetric) Is(target error) bool {
	var err ErrDecodeMetric
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrDecodeMetric) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrListEvalSets wraps a failure to list eval sets from the ADK launcher.
type ErrListEvalSets struct {
	Err error
}

func (e ErrListEvalSets) Error() string {
	return fmt.Sprintf("list eval sets: %v", e.Err)
}

func (e ErrListEvalSets) Unwrap() error { return e.Err }

func (e ErrListEvalSets) Is(target error) bool {
	var err ErrListEvalSets
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrListEvalSets) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}

// ErrRunEval wraps a failure to run an eval set via the ADK launcher.
type ErrRunEval struct {
	SetID string
	Err   error
}

func (e ErrRunEval) Error() string {
	return fmt.Sprintf("run_eval %s: %v", e.SetID, e.Err)
}

func (e ErrRunEval) Unwrap() error { return e.Err }

func (e ErrRunEval) Is(target error) bool {
	var err ErrRunEval
	return errors.As(target, &err) || errors.Is(e.Err, target)
}

func (e ErrRunEval) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Error())
}
