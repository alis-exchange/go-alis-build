package spanneradapter

import (
	"reflect"

	"cloud.google.com/go/spanner"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// ValueCodec is the internal seam that allows one consumer-owned table body
// to serve proto and non-proto resources. It handles scanning and value
// extraction for column data.
type ValueCodec[R any] interface {
	// NullDest returns a fresh scan destination for ColumnByName.
	NullDest() any
	// Value extracts a value from the destination, returning the value,
	// validity, and any error.
	Value(dest any) (val R, valid bool, err error)
}

// protoCodec is the internal implementation of ValueCodec for proto.Message types.
type protoCodec[R proto.Message] struct {
	elem reflect.Type
}

// ProtoCodec returns a ValueCodec for the given proto.Message type R.
func ProtoCodec[R proto.Message]() ValueCodec[R] {
	return protoCodec[R]{elem: reflect.TypeFor[R]().Elem()}
}

// NullDest returns a fresh spanner.NullProtoMessage with a new instance
// of the proto message type.
func (c protoCodec[R]) NullDest() any {
	return &spanner.NullProtoMessage{ProtoMessageVal: reflect.New(c.elem).Interface().(proto.Message)}
}

// Value extracts the proto.Message value from the NullProtoMessage destination.
func (c protoCodec[R]) Value(dest any) (R, bool, error) {
	var zero R
	d, ok := dest.(*spanner.NullProtoMessage)
	if !ok {
		return zero, false, status.Errorf(codes.Internal, "codec: wrong dest %T", dest)
	}
	if !d.Valid {
		return zero, false, nil
	}
	return d.ProtoMessageVal.(R), true, nil
}

// int64Codec is the internal implementation of ValueCodec for int64.
type int64Codec struct{}

// Int64Codec returns a ValueCodec for int64.
func Int64Codec() ValueCodec[int64] {
	return int64Codec{}
}

// NullDest returns a fresh spanner.NullInt64.
func (c int64Codec) NullDest() any {
	return &spanner.NullInt64{}
}

// Value extracts the int64 value from the NullInt64 destination.
func (c int64Codec) Value(dest any) (int64, bool, error) {
	d, ok := dest.(*spanner.NullInt64)
	if !ok {
		return 0, false, status.Errorf(codes.Internal, "codec: wrong dest %T", dest)
	}
	if !d.Valid {
		return 0, false, nil
	}
	return d.Int64, true, nil
}

// stringCodec is the internal implementation of ValueCodec for string.
type stringCodec struct{}

// StringCodec returns a ValueCodec for string.
func StringCodec() ValueCodec[string] {
	return stringCodec{}
}

// NullDest returns a fresh spanner.NullString.
func (c stringCodec) NullDest() any {
	return &spanner.NullString{}
}

// Value extracts the string value from the NullString destination.
func (c stringCodec) Value(dest any) (string, bool, error) {
	d, ok := dest.(*spanner.NullString)
	if !ok {
		return "", false, status.Errorf(codes.Internal, "codec: wrong dest %T", dest)
	}
	if !d.Valid {
		return "", false, nil
	}
	return d.StringVal, true, nil
}
