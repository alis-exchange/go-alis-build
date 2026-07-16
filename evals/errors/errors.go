package errors

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EvalError is implemented by evals package errors that map to gRPC status codes.
type EvalError interface {
	error
	Is(target error) bool
	GRPCStatus() *status.Status
}

// ToGRPC returns a gRPC-compatible error for err.
// EvalError values preserve their status code; other errors become InvalidArgument.
func ToGRPC(err error) error {
	if err == nil {
		return nil
	}
	if s, ok := evalStatus(err); ok {
		return s.Err()
	}
	return status.Error(codes.InvalidArgument, err.Error())
}

// ToGRPCf prefixes the eval error message with field for RPC validation errors.
func ToGRPCf(field string, err error) error {
	if err == nil {
		return nil
	}
	if s, ok := evalStatus(err); ok {
		return status.Errorf(s.Code(), "%s: %s", field, s.Message())
	}
	return status.Errorf(codes.InvalidArgument, "%s: %v", field, err)
}

// evalStatus extracts a gRPC status from err when it implements [EvalError].
// Returns false for plain errors so callers can fall back to InvalidArgument.
func evalStatus(err error) (*status.Status, bool) {
	var evalErr EvalError
	if errors.As(err, &evalErr) {
		return evalErr.GRPCStatus(), true
	}
	return nil, false
}

// IsEval reports whether err implements EvalError.
func IsEval(err error) bool {
	var evalErr EvalError
	return errors.As(err, &evalErr)
}

// Code returns the gRPC code for err when it implements EvalError.
func Code(err error) codes.Code {
	if s, ok := evalStatus(err); ok {
		return s.Code()
	}
	return codes.Unknown
}
