package loadinfra

import (
	"context"
	"errors"
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestEscapeFilterLabel(t *testing.T) {
	t.Parallel()

	got := escapeFilterLabel(`svc"beta\1`)
	want := `svc\"beta\\1`
	if got != want {
		t.Fatalf("escapeFilterLabel = %q, want %q", got, want)
	}
}

func TestClassifyFetchStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want evalspb.InfraFetchStatus
	}{
		{name: "nil", err: nil, want: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK},
		{name: "deadline", err: context.DeadlineExceeded, want: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_TIMEOUT},
		{name: "grpc permission", err: status.Error(codes.PermissionDenied, "denied"), want: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_PERMISSION_DENIED},
		{name: "grpc deadline", err: status.Error(codes.DeadlineExceeded, "slow"), want: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_TIMEOUT},
		{name: "google 403", err: &googleapi.Error{Code: 403}, want: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_PERMISSION_DENIED},
		{name: "google 408", err: &googleapi.Error{Code: 408}, want: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_TIMEOUT},
		{name: "generic", err: errors.New("boom"), want: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyFetchStatus(tc.err); got != tc.want {
				t.Fatalf("classifyFetchStatus(%v)=%v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
