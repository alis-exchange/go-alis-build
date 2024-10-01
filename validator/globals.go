package validator

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var validators = make(map[string]map[string]*Validator)

// Validate the proto message. Use this function in your server interceptor.
func Validate(ctx context.Context, request interface{}) (error, bool) {
	v, msg, err := locateValidator(ctx, request)
	if err != nil {
		return err, false
	}
	return v.Validate(msg, []string{}), true
}

func locateValidator(ctx context.Context, request interface{}) (*Validator, protoreflect.ProtoMessage, error) {
	protoMsg, ok := request.(protoreflect.ProtoMessage)
	if !ok {
		return nil, nil, status.Errorf(codes.Internal, "invalid request")
	}
	msgType := getMsgType(protoMsg)
	v, err := getValidator(ctx, msgType)
	if err != nil {
		return nil, nil, err
	}
	return v, protoMsg, nil
}

func getMsgType(msg protoreflect.ProtoMessage) string {
	return string(msg.ProtoReflect().Descriptor().FullName())
}

func getValidator(ctx context.Context, msgType string) (*Validator, error) {
	typeValidators, ok := validators[msgType]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "no validators found for %s", msgType)
	}
	var v *Validator
	rpc, ok := grpc.Method(ctx)
	if !ok {
		v, ok = typeValidators["*"]
		if !ok {
			return nil, status.Errorf(codes.Internal, "could not get rpc method")
		}
	} else {
		svcName := strings.Split(rpc, "/")[1]
		v, ok = typeValidators[svcName]
		if !ok {
			v, ok = typeValidators["*"]
			if !ok {
				return nil, status.Errorf(codes.NotFound, "no validators found for %s in %s", msgType, svcName)
			}
		}
	}
	return v, nil
}
