package spanneradapter

import (
	"testing"

	"cloud.google.com/go/spanner"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestProtoCodecRoundtrip(t *testing.T) {
	c := ProtoCodec[*fieldmaskpb.FieldMask]()
	d := c.NullDest()
	npm := d.(*spanner.NullProtoMessage)
	npm.ProtoMessageVal.(*fieldmaskpb.FieldMask).Paths = []string{"x"}
	npm.Valid = true
	v, ok, err := c.Value(d)
	if err != nil || !ok || v.Paths[0] != "x" {
		t.Fatalf("%v %v %v", v, ok, err)
	}
}

func TestProtoCodecFreshDestPerCall(t *testing.T) {
	c := ProtoCodec[*fieldmaskpb.FieldMask]()
	if c.NullDest() == c.NullDest() {
		t.Fatal("dest must be fresh per scan")
	}
}

func TestInt64Codec(t *testing.T) {
	c := Int64Codec()
	d := c.NullDest().(*spanner.NullInt64)
	d.Int64, d.Valid = 42, true
	v, ok, _ := c.Value(d)
	if !ok || v != 42 {
		t.Fatal(v)
	}
}
