package spending

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func withInfo(t *testing.T, code codes.Code, message string, info *errdetails.ErrorInfo) error {
	t.Helper()
	s, err := status.New(code, message).WithDetails(info)
	if err != nil {
		t.Fatal(err)
	}
	return s.Err()
}

func TestFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Refusal
		ok   bool
	}{
		{
			name: "what the guard returns",
			err:  Refused("MEMBER_LIMIT_REACHED", "accounts/a1", "You have reached your usage limit on this account."),
			want: Refusal{Reason: "MEMBER_LIMIT_REACHED", Account: "accounts/a1", Comments: "You have reached your usage limit on this account."},
			ok:   true,
		},
		{
			name: "wrapped on its way up",
			err:  fmt.Errorf("running the turn: %w", Refused("NO_FUNDING", "accounts/a1", "Add credits or valid billing details to continue.")),
			want: Refusal{Reason: "NO_FUNDING", Account: "accounts/a1", Comments: "Add credits or valid billing details to continue."},
			ok:   true,
		},
		{
			name: "ideas, on its own domain and code",
			err: withInfo(t, codes.ResourceExhausted, "Your account has reached its credit limit.", &errdetails.ErrorInfo{
				Reason: "CREDIT_LIMIT_REACHED", Domain: "alis.build", Metadata: map[string]string{"account": "accounts/a1"},
			}),
			want: Refusal{Reason: "CREDIT_LIMIT_REACHED", Account: "accounts/a1", Comments: "Your account has reached its credit limit."},
			ok:   true,
		},
		{
			name: "ideas, restated by a service in between with the details kept",
			err: withInfo(t, codes.ResourceExhausted, "adding the note to ideas/i1: rpc error: code = ResourceExhausted desc = Your account has reached its credit limit.", &errdetails.ErrorInfo{
				Reason: "CREDIT_LIMIT_REACHED", Domain: "alis.build", Metadata: map[string]string{"account": "accounts/a1"},
			}),
			want: Refusal{Reason: "CREDIT_LIMIT_REACHED", Account: "accounts/a1", Comments: "Your account has reached its credit limit."},
			ok:   true,
		},
		{
			name: "another alis.build reason is not a refusal",
			err:  withInfo(t, codes.FailedPrecondition, "no goal", &errdetails.ErrorInfo{Reason: "NO_PROJECT_GOAL", Domain: "alis.build"}),
		},
		{name: "a bare denial", err: status.Error(codes.PermissionDenied, "no")},
		{name: "a rate limit", err: status.Error(codes.ResourceExhausted, "slow down")},
		{name: "not a status", err: errors.New("boom")},
		{name: "nil"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := FromError(test.err)
			if ok != test.ok || got != test.want {
				t.Fatalf("FromError() = %+v, %v; want %+v, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestRefusedKeepsTheCodeAndMessageOlderReadersUse(t *testing.T) {
	s := status.Convert(Refused("NO_FUNDING", "", ""))
	if s.Code() != codes.PermissionDenied || s.Message() != "NO_FUNDING: This payer does not currently allow chargeable work." {
		t.Fatalf("got %v %q", s.Code(), s.Message())
	}
}
