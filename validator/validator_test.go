package validator

import (
	"context"
	"testing"
	"time"

	"go.alis.build/alog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

var val *Validator

func init() {
	// SF := StringField
	// SV := StringValue
	// IF := IntField
	// IV := IntValue

	subMVal := NewValidator(&pbOpen.Test_SubMessage{}, nil)
	subMVal.StringGetter = func(data protoreflect.ProtoMessage, path string) (string, error) {
		msg := data.(*pbOpen.Test_SubMessage)
		switch path {
		case "string":
			return msg.String_, nil
		}
		return "", status.Error(codes.Internal, "invalid path")
	}
	subMVal.AddRule(StringField("string").Length().Equals(Int(2)))

	val = NewValidator(&pbOpen.Test{}, &ValidatorOptions{})

	// add getters to avoid reflection which will chow cpu
	val.StringGetter = func(data protoreflect.ProtoMessage, path string) (string, error) {
		msg := data.(*pbOpen.Test)
		switch path {
		case "name":
			return msg.Name, nil
		case "display_name":
			return msg.DisplayName, nil
		}
		return "", status.Error(codes.Internal, "invalid path")
	}
	val.IntGetter = func(data protoreflect.ProtoMessage, path string) (int64, error) {
		msg := data.(*pbOpen.Test)
		switch path {
		case "int32":
			return int64(msg.Int32), nil
		case "int64":
			return msg.Int64, nil
		}
		return 0, status.Error(codes.Internal, "invalid path")
	}
	val.FloatGetter = func(data protoreflect.ProtoMessage, path string) (float64, error) {
		msg := data.(*pbOpen.Test)
		switch path {
		case "float":
			return float64(msg.Float), nil
		}
		return 0, status.Error(codes.Internal, "invalid path")
	}
	val.SubMessageGetter = func(data protoreflect.ProtoMessage, path string) (protoreflect.ProtoMessage, error) {
		msg := data.(*pbOpen.Test)
		switch path {
		case "sub_message":
			return msg.SubMessage, nil
		}
		return nil, status.Error(codes.Internal, "invalid path")
	}
	val.EnumGetter = func(data protoreflect.ProtoMessage, path string) (protoreflect.EnumNumber, error) {
		msg := data.(*pbOpen.Test)
		switch path {
		case "test_enum":
			return protoreflect.EnumNumber(msg.TestEnum), nil
		}
		return 0, status.Error(codes.Internal, "invalid path")
	}
	val.StringListGetter = func(data protoreflect.ProtoMessage, path string) ([]string, error) {
		msg := data.(*pbOpen.Test)
		switch path {
		case "repeated_string":
			return msg.RepeatedString, nil
		}
		return nil, status.Error(codes.Internal, "invalid path")
	}

	rule := SubMsgListLength("repeated_sub_message").Equals(Int(1))
	val.AddRule(rule)
	val.AddSubMessageValidator("repeated_sub_message", subMVal, &SubMsgOptions{IsRepeated: true})
}

func TestValidate(t *testing.T) {
	startT := time.Now()
	m := &pbOpen.Test{
		DisplayName:    "ab",
		RepeatedString: []string{"string"},
		SubMessage: &pbOpen.Test_SubMessage{
			String_: "ab",
		},
		RepeatedBool:   []bool{true},
		RepeatedDouble: []float64{1},
		RepeatedSubMessage: []*pbOpen.Test_SubMessage{
			{
				String_: "abd",
			},
			{
				String_: "abc",
			},
		},
	}
	err := val.Validate(m, []string{})
	alog.Infof(context.Background(), "time: %v", time.Since(startT))
	t.Logf("err: %v", err)
}

func TestRetrieveRulesRpc(t *testing.T) {
	type args struct {
		req *pbOpen.RetrieveRulesRequest
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "TestRetrieveRulesRpc",
			args: args{
				req: &pbOpen.RetrieveRulesRequest{
					MsgType: "alis.open.validation.v1.Test",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RetrieveRulesRpc(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("RetrieveRulesRpc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Logf("got: %v", got)
		})
	}
}
