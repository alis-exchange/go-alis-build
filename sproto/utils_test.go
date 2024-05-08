package sproto

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
)

func Test_newEmptyMessage(t *testing.T) {
	type args struct {
		msg proto.Message
	}
	tests := []struct {
		name string
		args args
		want proto.Message
	}{
		{
			name: "Test_newEmptyMessage",
			args: args{
				msg: &fieldmaskpb.FieldMask{},
			},
			want: &fieldmaskpb.FieldMask{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newEmptyMessage(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newEmptyMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseStructPbValue(t *testing.T) {
	type args struct {
		value *structpb.Value
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		{
			name: "Test_parseStructPbValue_StringValue",
			args: args{
				value: &structpb.Value{
					Kind: &structpb.Value_StringValue{
						StringValue: "test",
					},
				},
			},
			want: "test",
		},
		{
			name: "Test_parseStructPbValue_NullValue",
			args: args{
				value: &structpb.Value{
					Kind: &structpb.Value_NullValue{
						NullValue: 0,
					},
				},
			},
			want: nil,
		},
		{
			name: "Test_parseStructPbValue_NumberValue",
			args: args{
				value: &structpb.Value{
					Kind: &structpb.Value_NumberValue{
						NumberValue: 1,
					},
				},
			},
			want: 1.0,
		},
		{
			name: "Test_parseStructPbValue_BoolValue",
			args: args{
				value: &structpb.Value{
					Kind: &structpb.Value_BoolValue{
						BoolValue: true,
					},
				},
			},
			want: true,
		},
		{
			name: "Test_parseStructPbValue_ListValue",
			args: args{
				value: &structpb.Value{
					Kind: &structpb.Value_ListValue{
						ListValue: &structpb.ListValue{
							Values: []*structpb.Value{
								{
									Kind: &structpb.Value_StringValue{
										StringValue: "test",
									},
								},
								{
									Kind: &structpb.Value_NumberValue{
										NumberValue: 1,
									},
								},
								{
									Kind: &structpb.Value_BoolValue{
										BoolValue: true,
									},
								},
							},
						},
					},
				},
			},
			want: []interface{}{
				"test",
				1.0,
				true,
			},
		},
		{
			name: "Test_parseStructPbValue_StructValue",
			args: args{
				value: &structpb.Value{
					Kind: &structpb.Value_StructValue{
						StructValue: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"string": {
									Kind: &structpb.Value_StringValue{
										StringValue: "test",
									},
								},
								"number": {
									Kind: &structpb.Value_NumberValue{
										NumberValue: 1,
									},
								},
								"bool": {
									Kind: &structpb.Value_BoolValue{
										BoolValue: true,
									},
								},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"string": "test",
				"number": 1.0,
				"bool":   true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseStructPbValue(tt.args.value); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseStructPbValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
