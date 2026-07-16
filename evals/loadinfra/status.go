package loadinfra

import (
	"context"
	"errors"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// diagnosticTargetID is the synthetic target id on [ConfigFailureSnapshot].
const diagnosticTargetID = "_evals"

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
func classifyFetchStatus(err error) evalspb.InfraFetchStatus {
	if err == nil {
		return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_TIMEOUT
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.PermissionDenied:
			return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_PERMISSION_DENIED
		case codes.DeadlineExceeded:
			return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_TIMEOUT
		}
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		switch gerr.Code {
		case 403:
			return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_PERMISSION_DENIED
		case 408:
			return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_TIMEOUT
		}
	}
	return evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE
}
