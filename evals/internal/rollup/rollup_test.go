package rollup_test

import (
	"testing"

	evalspb "go.alis.build/common/alis/evals"
	"go.alis.build/evals/internal/rollup"
)

// TestStatus checks the FAILED > NOT_EVALUATED > PASSED precedence.
func TestStatus(t *testing.T) {
	t.Parallel()

	identity := func(s evalspb.Status) evalspb.Status { return s }
	tests := []struct {
		name string
		in   []evalspb.Status
		want evalspb.Status
	}{
		{name: "empty", in: nil, want: evalspb.Status_PASSED},
		{name: "all passed", in: []evalspb.Status{evalspb.Status_PASSED, evalspb.Status_PASSED}, want: evalspb.Status_PASSED},
		{
			name: "not evaluated",
			in:   []evalspb.Status{evalspb.Status_PASSED, evalspb.Status_NOT_EVALUATED},
			want: evalspb.Status_NOT_EVALUATED,
		},
		{name: "failed wins", in: []evalspb.Status{evalspb.Status_NOT_EVALUATED, evalspb.Status_FAILED}, want: evalspb.Status_FAILED},
		{
			name: "failed before not evaluated",
			in:   []evalspb.Status{evalspb.Status_FAILED, evalspb.Status_NOT_EVALUATED},
			want: evalspb.Status_FAILED,
		},
		{name: "unspecified ignored", in: []evalspb.Status{evalspb.Status_STATUS_UNSPECIFIED}, want: evalspb.Status_PASSED},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := rollup.Status(tt.in, identity); got != tt.want {
				t.Fatalf("Status(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
