package protodb_test

import (
	"reflect"
	"testing"

	protodb "go.alis.build/protodb/v2"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// TestMergeRespectsPaths checks that Merge replaces a listed repeated field
// in dst with the value from src rather than appending to it.
func TestMergeRespectsPaths(t *testing.T) {
	dst := &fieldmaskpb.FieldMask{Paths: []string{"old"}}
	src := &fieldmaskpb.FieldMask{Paths: []string{"new"}}
	protodb.Merge(dst, src, "paths")
	if !reflect.DeepEqual(dst.Paths, []string{"new"}) {
		t.Fatalf("got %v", dst.Paths)
	}
}

// TestMergeDoesNotMutateSrc checks that Merge leaves the src message unchanged.
func TestMergeDoesNotMutateSrc(t *testing.T) {
	src := &fieldmaskpb.FieldMask{Paths: []string{"a"}}
	protodb.Merge(&fieldmaskpb.FieldMask{}, src, "paths")
	if len(src.Paths) != 1 {
		t.Fatal("src mutated")
	}
}

// TestApplyReadMaskDoesNotMutateMask checks that extra paths passed to
// ApplyReadMask are not appended to the caller's mask.
func TestApplyReadMaskDoesNotMutateMask(t *testing.T) {
	msg := &fieldmaskpb.FieldMask{Paths: []string{"x"}}
	mask := &fieldmaskpb.FieldMask{Paths: []string{"paths"}}
	if err := protodb.ApplyReadMask(msg, mask, "ignored_extra"); err != nil {
		t.Fatal(err)
	}
	if len(mask.Paths) != 1 || mask.Paths[0] != "paths" {
		t.Fatalf("caller's mask mutated: %v", mask.Paths)
	}
}

// TestApplyReadMaskNilMaskIsNoop checks that a nil mask leaves the message
// unfiltered and returns no error.
func TestApplyReadMaskNilMaskIsNoop(t *testing.T) {
	msg := &fieldmaskpb.FieldMask{Paths: []string{"x"}}
	if err := protodb.ApplyReadMask(msg, nil); err != nil {
		t.Fatal(err)
	}
	if len(msg.Paths) != 1 {
		t.Fatal("nil mask must not filter")
	}
}
