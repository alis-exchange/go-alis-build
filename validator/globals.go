package validator

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var validators = make(map[string]*Validator)

// Validate the proto message. Use this function in your server interceptor.
func Validate(request interface{}) (error, bool) {
	v, msg, err := locateValidator(request)
	if err != nil {
		return err, false
	}
	return v.Validate(msg, []string{}), true
}

func locateValidator(request interface{}) (*Validator, protoreflect.ProtoMessage, error) {
	protoMsg, ok := request.(protoreflect.ProtoMessage)
	if !ok {
		return nil, nil, status.Errorf(codes.Internal, "invalid request")
	}
	v, ok := validators[getMsgType(protoMsg)]
	if !ok {
		return nil, nil, status.Errorf(codes.NotFound, "validator not found")
	}
	return v, protoMsg, nil
}

func getMsgType(msg protoreflect.ProtoMessage) string {
	return string(msg.ProtoReflect().Descriptor().FullName())
}
