package loadinfra

import (
	"context"
	"errors"
	"fmt"
	"testing"

	evalspb "go.alis.build/common/alis/evals"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClassifyFetchStatus_permissionDeniedWinsOverTimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want evalspb.InfraFetchStatus
	}{
		{
			"timeout then denied",
			joinErrors(status.Error(codes.DeadlineExceeded, "slow"), status.Error(codes.PermissionDenied, "denied")),
			evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_PERMISSION_DENIED,
		},
		{
			"http 408 then http 403",
			joinErrors(&googleapi.Error{Code: 408}, &googleapi.Error{Code: 403}),
			evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_PERMISSION_DENIED,
		},
		{
			"no data then context deadline",
			joinErrors(noData("a"), fmt.Errorf("b: %w", context.DeadlineExceeded)),
			evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_TIMEOUT,
		},
		{"no data only", joinErrors(noData("a"), noData("b")), evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE},
		{"plain error", errors.New("boom"), evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE},
		{"nil", nil, evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyFetchStatus(tt.err); got != tt.want {
				t.Fatalf("classifyFetchStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJoinErrors_keepsSemicolonText(t *testing.T) {
	t.Parallel()
	err := joinErrors(errors.New("a: no data"), nil, errors.New("b: denied"))
	if got, want := err.Error(), "a: no data; b: denied"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
