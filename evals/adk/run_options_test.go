package adk_test

import (
	"testing"

	"go.alis.build/evals/adk"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestSessionStateFromProto_nil(t *testing.T) {
	t.Parallel()

	got, err := adk.SessionStateFromProto(nil)
	if err != nil {
		t.Fatalf("SessionStateFromProto() error = %v", err)
	}
	if got != nil {
		t.Fatalf("got = %#v, want nil", got)
	}
}

func TestSessionStateFromProto_empty(t *testing.T) {
	t.Parallel()

	got, err := adk.SessionStateFromProto(&structpb.Struct{})
	if err != nil {
		t.Fatalf("SessionStateFromProto() error = %v", err)
	}
	if got != nil {
		t.Fatalf("got = %#v, want nil", got)
	}
}

func TestSessionStateFromProto_values(t *testing.T) {
	t.Parallel()

	s, err := structpb.NewStruct(map[string]any{
		"idea_name":    "ideas/abc",
		"account_name": "accounts/xyz",
		"enabled":      true,
	})
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}

	got, err := adk.SessionStateFromProto(s)
	if err != nil {
		t.Fatalf("SessionStateFromProto() error = %v", err)
	}
	if got["idea_name"] != "ideas/abc" {
		t.Fatalf("idea_name = %v", got["idea_name"])
	}
	if got["account_name"] != "accounts/xyz" {
		t.Fatalf("account_name = %v", got["account_name"])
	}
	if got["enabled"] != true {
		t.Fatalf("enabled = %v", got["enabled"])
	}
}
