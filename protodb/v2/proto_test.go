package protodb_test

import (
	"reflect"
	"testing"

	protodb "go.alis.build/protodb/v2"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestMergeRespectsPaths(t *testing.T) {
	dst := &fieldmaskpb.FieldMask{Paths: []string{"old"}}
	src := &fieldmaskpb.FieldMask{Paths: []string{"new"}}
	protodb.Merge(dst, src, "paths")
	if !reflect.DeepEqual(dst.Paths, []string{"new"}) {
		t.Fatalf("got %v", dst.Paths)
	}
}

func TestMergeDoesNotMutateSrc(t *testing.T) {
	src := &fieldmaskpb.FieldMask{Paths: []string{"a"}}
	protodb.Merge(&fieldmaskpb.FieldMask{}, src, "paths")
	if len(src.Paths) != 1 {
		t.Fatal("src mutated")
	}
}

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

func TestApplyReadMaskNilMaskIsNoop(t *testing.T) {
	msg := &fieldmaskpb.FieldMask{Paths: []string{"x"}}
	if err := protodb.ApplyReadMask(msg, nil); err != nil {
		t.Fatal(err)
	}
	if len(msg.Paths) != 1 {
		t.Fatal("nil mask must not filter")
	}
}
