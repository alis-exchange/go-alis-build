package validator

import (
	"testing"
	"time"

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

	rule := StringField("display_name").Length().Equals(IntValue(4))
	// rule := NewRule(&Rule{
	// 	fieldPaths:     []string{"name"},
	// 	isViolated:     isViolated,
	// 	Description:    "name must be test",
	// 	NotDescription: "name must not be test",
	// })
	val.AddRule(OR(rule, StringField("display_name").Equals(StringValue("test"))))
	// val.AddRule(IntField("int32").Plus(IntField("int64")).Equals(IntValue(2)))
}

func isViolated(data protoreflect.ProtoMessage) (bool, error) {
	msg := data.(*pbOpen.Test)
	if msg.Name == "test" {
		return false, nil
	}

	return true, nil
}

func Test_Validate(t *testing.T) {
	msg := &pbOpen.Test{Name: "test", Int32: 1, Int64: 1}
	startT := time.Now()
	err := val.Validate(msg)
	t.Logf("time: %v", time.Since(startT))
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
