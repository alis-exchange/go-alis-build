package filtering

import (
	"testing"
	"time"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// newDate builds a google.type.Date message without importing genproto, from
// a hand-written descriptor with the same full name and fields.
func newDate(t *testing.T, year, month, day int32) proto.Message {
	t.Helper()
	i32 := descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()
	opt := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:    proto.String("google/type/date_test.proto"),
		Package: proto.String("google.type"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Date"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("year"), Number: proto.Int32(1), Type: i32, Label: opt, JsonName: proto.String("year")},
				{Name: proto.String("month"), Number: proto.Int32(2), Type: i32, Label: opt, JsonName: proto.String("month")},
				{Name: proto.String("day"), Number: proto.Int32(3), Type: i32, Label: opt, JsonName: proto.String("day")},
			},
		}},
	}, nil)
	require.NoError(t, err)
	m := dynamicpb.NewMessage(fd.Messages().ByName("Date"))
	f := m.Descriptor().Fields()
	m.Set(f.ByName("year"), protoreflect.ValueOfInt32(year))
	m.Set(f.ByName("month"), protoreflect.ValueOfInt32(month))
	m.Set(f.ByName("day"), protoreflect.ValueOfInt32(day))
	return m
}

func TestPredicateProtoValues(t *testing.T) {
	p, err := NewParser(
		Timestamp("Backup.expire_time"),
		EnumString("Backup.state", "google.spanner.admin.database.v1.Backup.State"),
		Duration("ttl"),
		Date("day"),
		EnumInteger("num_state", "google.spanner.admin.database.v1.Backup.State"),
	)
	require.NoError(t, err)
	ready := databasepb.Backup_READY.Descriptor().Values().ByNumber(protoreflect.EnumNumber(databasepb.Backup_READY))
	set := rowOf(map[string]any{
		"Backup.expire_time": timestamppb.New(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)),
		"Backup.state":       ready,
		"plain_state":        ready,
		"num_state":          ready,
		"Backup.create_time": timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)), // unregistered WKT
		"ttl":                durationpb.New(90 * time.Minute),
		"day":                newDate(t, 2024, 5, 1),
	})
	unset := rowOf(map[string]any{"Backup.expire_time": nil, "Backup.state": ready})
	cases := []struct {
		filter string
		row    func(string) (any, error)
		want   bool
	}{
		{"Backup.expire_time = NULL", unset, true},
		{"Backup.expire_time = NULL", set, false},
		{"Backup.expire_time != NULL", set, true},
		{"Backup.expire_time > timestamp('2029-01-01T00:00:00Z')", set, true},
		{"Backup.create_time < timestamp('2025-01-01T00:00:00Z')", set, true},
		{"Backup.state = 'READY'", set, true},
		{"Backup.state = 'CREATING'", set, false},
		{"Backup.state = 'READY' AND Backup.expire_time = NULL", unset, true},
		{"ttl > duration('1h')", set, true},
		{"ttl < duration('1h')", set, false},
		{"day = date('2024-05-01')", set, true},
		{"plain_state = 'READY'", set, true},
		{"plain_state = 2", set, true},
		{"plain_state = 1", set, false},
		{"num_state = 2", set, true},
		{"plain_state IN ['CREATING', 'READY']", set, true},
	}
	for _, tc := range cases {
		pred, err := p.Compile(tc.filter)
		require.NoError(t, err, tc.filter)
		got, err := pred.Match(tc.row)
		require.NoError(t, err, tc.filter)
		assert.Equal(t, tc.want, got, tc.filter)
	}
}

func TestPredicateUnsupportedMessageComparison(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	row := rowOf(map[string]any{"Backup.encryption_info": &databasepb.EncryptionInfo{}})
	pred, err := p.Compile("Backup.encryption_info = 'x'")
	require.NoError(t, err)
	_, err = pred.Match(row)
	assert.Equal(t, codes.Unimplemented, status.Code(err))

	// A null test on a set message works: it is simply not null.
	pred, err = p.Compile("Backup.encryption_info != NULL")
	require.NoError(t, err)
	got, err := pred.Match(row)
	require.NoError(t, err)
	assert.True(t, got)
}
