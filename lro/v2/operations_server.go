package lro

import (
	"context"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OperationsServer serves the standard google.longrunning.Operations RPCs using an LRO client.
type OperationsServer struct {
	longrunningpb.UnimplementedOperationsServer
	client           *Client
	defaultTimeout   time.Duration
	initialPollDelay time.Duration
	maxPollDelay     time.Duration
}

// OperationsServerOption configures an OperationsServer.
type OperationsServerOption func(*OperationsServer)

// WithDefaultWaitTimeout sets the default timeout used when WaitOperation does not specify one.
func WithDefaultWaitTimeout(timeout time.Duration) OperationsServerOption {
	return func(server *OperationsServer) {
		server.defaultTimeout = timeout
	}
}

// WithWaitPolling configures the initial and maximum polling intervals for WaitOperation.
func WithWaitPolling(initialDelay, maxDelay time.Duration) OperationsServerOption {
	return func(server *OperationsServer) {
		server.initialPollDelay = initialDelay
		server.maxPollDelay = maxDelay
	}
}

// NewOperationsServer constructs a google.longrunning.Operations server backed by the client.
func NewOperationsServer(client *Client, opts ...OperationsServerOption) *OperationsServer {
	server := &OperationsServer{
		client:           client,
		defaultTimeout:   10 * time.Minute,
		initialPollDelay: 1 * time.Second,
		maxPollDelay:     10 * time.Second,
	}
	for _, opt := range opts {
		opt(server)
	}
	return server
}

// GetOperation retrieves the latest state of a long-running operation.
func (s *OperationsServer) GetOperation(ctx context.Context, req *longrunningpb.GetOperationRequest) (*longrunningpb.Operation, error) {
	if s.client == nil {
		return nil, status.Error(codes.FailedPrecondition, "lro client is required")
	}
	if err := validateOperationName(req.GetName()); err != nil {
		return nil, err
	}
	op, err := s.client.GetOperationPb(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	return op, nil
}

// WaitOperation waits until the operation is done or the effective timeout elapses.
func (s *OperationsServer) WaitOperation(ctx context.Context, req *longrunningpb.WaitOperationRequest) (*longrunningpb.Operation, error) {
	if s.client == nil {
		return nil, status.Error(codes.FailedPrecondition, "lro client is required")
	}
	if err := validateOperationName(req.GetName()); err != nil {
		return nil, err
	}

	timeout := s.defaultTimeout
	if req.GetTimeout() != nil {
		timeout = req.GetTimeout().AsDuration()
	}
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	operation, err := s.GetOperation(ctx, &longrunningpb.GetOperationRequest{Name: req.GetName()})
	if err != nil {
		return nil, err
	}

	sleepDuration := s.initialPollDelay
	if sleepDuration <= 0 {
		sleepDuration = time.Second
	}
	maxSleepDuration := s.maxPollDelay
	if maxSleepDuration <= 0 {
		maxSleepDuration = 10 * time.Second
	}

	for !operation.GetDone() {
		select {
		case <-ctx.Done():
			return operation, ctx.Err()
		case <-time.After(sleepDuration):
			operation, err = s.GetOperation(ctx, &longrunningpb.GetOperationRequest{Name: req.GetName()})
			if err != nil {
				return nil, err
			}
			if sleepDuration < maxSleepDuration {
				sleepDuration *= 2
				if sleepDuration > maxSleepDuration {
					sleepDuration = maxSleepDuration
				}
			}
		}
	}

	return operation, nil
}
