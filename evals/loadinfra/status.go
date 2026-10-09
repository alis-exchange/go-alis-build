package loadinfra

import (
	"context"
	"errors"
	"net/http"

	evalspb "go.alis.build/common/alis/evals"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// diagnosticTargetID is the synthetic target id on [ConfigFailureSnapshot].
const diagnosticTargetID = "_evals.diagnostic"

// ConfigFailureSnapshot returns a synthetic Cloud Run snapshot carrying a
// configuration or setup error when no real targets were observed.
func ConfigFailureSnapshot(message string) *evalspb.CloudRunTargetSnapshot {
	msg := message
	return &evalspb.CloudRunTargetSnapshot{
		Id:           diagnosticTargetID,
		FetchStatus:  evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE,
		FetchMessage: &msg,
	}
}

// classifyFetchStatus maps API and context errors to InfraFetchStatus values.
// It inspects every joined and wrapped error: any permission denial wins,
// then any timeout; everything else is UNAVAILABLE.
func classifyFetchStatus(err error) evalspb.InfraFetchStatus {
	switch {
	case err == nil:
		return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK
	case anyLeaf(err, isPermissionDenied):
		return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_PERMISSION_DENIED
	case anyLeaf(err, isTimeout):
		return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_TIMEOUT
	default:
		return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE
	}
}

// anyLeaf reports whether pred holds for err or any error it wraps through
// Unwrap() error or Unwrap() []error. classifyFetchStatus runs one full walk
// per status, so a denial anywhere wins over a timeout anywhere; a single
// top-level errors.As would stop at the first match and break that order.
func anyLeaf(err error, pred func(error) bool) bool {
	if err == nil {
		return false
	}
	if pred(err) {
		return true
	}
	switch u := err.(type) { //nolint:errorlint // anyLeaf visits every node itself, so a type switch per node is intended.
	case interface{ Unwrap() []error }:
		for _, inner := range u.Unwrap() {
			if anyLeaf(inner, pred) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return anyLeaf(u.Unwrap(), pred)
	}
	return false
}

// grpcStatusError is implemented by errors that carry a gRPC status.
type grpcStatusError interface{ GRPCStatus() *status.Status }

// isPermissionDenied matches gRPC PermissionDenied or HTTP 403 at or below
// err. anyLeaf calls it on every node.
func isPermissionDenied(err error) bool {
	var gs grpcStatusError
	if errors.As(err, &gs) && gs.GRPCStatus().Code() == codes.PermissionDenied {
		return true
	}
	var g *googleapi.Error
	return errors.As(err, &g) && g.Code == http.StatusForbidden
}

// isTimeout matches a context deadline, gRPC DeadlineExceeded or HTTP 408 at
// or below err. anyLeaf calls it on every node.
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var gs grpcStatusError
	if errors.As(err, &gs) && gs.GRPCStatus().Code() == codes.DeadlineExceeded {
		return true
	}
	var g *googleapi.Error
	return errors.As(err, &g) && g.Code == http.StatusRequestTimeout
}
