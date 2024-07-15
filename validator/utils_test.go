package validator

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	pb "internal.os.alis.services/protobuf/alis/os/resources/products/v1"
)

func Test_getStringField(t *testing.T) {
	type args struct {
		msg       protoreflect.ProtoMessage
		fieldName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Test getStringField",
			args: args{
				msg: &pb.Product{
					Name: "asdf",
					BuildConfig: &pb.Product_BuildConfig{
						Repository: "asdf",
					},
				},
				fieldName: "state",
			},
			want: "asdf",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getStringField(tt.args.msg, tt.args.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("getStringField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getStringField() = %v, want %v", got, tt.want)
			}
		})
	}
}
