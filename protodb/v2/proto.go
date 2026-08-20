package protodb

import (
	"github.com/mennanov/fmutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// Merge overwrites the fields of dst named by paths with src's values for
// those fields. src is cloned first and never mutated.
func Merge(dst, src proto.Message, paths ...string) {
	cloned := proto.Clone(src)
	fmutils.Filter(cloned, paths)
	fmutils.Prune(dst, paths)
	proto.Merge(dst, cloned)
}

// ApplyReadMask filters msg down to the fields named by mask (plus
// ignoredPaths, which always survive). mask is cloned — the caller's mask is
// never modified, so a mask may be reused across rows. A nil mask is a no-op.
func ApplyReadMask(msg proto.Message, mask *fieldmaskpb.FieldMask, ignoredPaths ...string) error {
	if mask == nil {
		return nil
	}
	if !mask.IsValid(msg) {
		return status.Errorf(codes.InvalidArgument, "invalid read mask: %v", mask)
	}
	m := proto.Clone(mask).(*fieldmaskpb.FieldMask)
	m.Paths = append(m.Paths, ignoredPaths...)
	m.Normalize()
	fmutils.Filter(msg, m.GetPaths())
	return nil
}
