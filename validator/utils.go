package validator

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func GetMsgType(msg protoreflect.ProtoMessage) string {
	return string(msg.ProtoReflect().Descriptor().FullName())
}

func GetIntField(data interface{}, fieldName string) (int64, error) {
	msg := data.(protoreflect.ProtoMessage)
	parentMsg, fieldName, err := GetFieldParentMessage(msg, fieldName)
	if err != nil {
		return 0, err
	}
	md := parentMsg.ProtoReflect().Descriptor()

	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if fd.TextName() == fieldName {
			if fd.Kind() == protoreflect.Int32Kind || fd.Kind() == protoreflect.Int64Kind {
				value := parentMsg.ProtoReflect().Get(fd).Int()
				return value, nil
			} else {
				return 0, status.Errorf(codes.InvalidArgument, "%s is not an integer field", fieldName)
			}
		}
	}

	return 0, status.Errorf(codes.Internal, "%s not found", fieldName)
}

func GetFloatField(data interface{}, fieldName string) (float64, error) {
	msg := data.(protoreflect.ProtoMessage)
	parentMsg, fieldName, err := GetFieldParentMessage(msg, fieldName)
	if err != nil {
		return 0, err
	}
	md := parentMsg.ProtoReflect().Descriptor()

	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if fd.TextName() == fieldName {
			if fd.Kind() == protoreflect.FloatKind || fd.Kind() == protoreflect.DoubleKind {
				value := parentMsg.ProtoReflect().Get(fd).Float()
				return value, nil
			} else {
				return 0, status.Errorf(codes.InvalidArgument, "%s is not a float field", fieldName)
			}
		}
	}

	return 0, status.Errorf(codes.Internal, "%s not found", fieldName)
}

func GetFieldParentMessage(msg protoreflect.ProtoMessage, path string) (protoreflect.ProtoMessage, string, error) {
	pathParts := strings.Split(path, ".")
	if len(pathParts) == 1 {
		return msg, path, nil
	}
	// remove last part
	for j, part := range pathParts {
		// if part does not contain alphanumeric or underscore, return error
		if !IsAlphanumericOrUnderscore(part) {
			return nil, "", status.Errorf(codes.InvalidArgument, "invalid field name: %s", part)
		}
		md := msg.ProtoReflect().Descriptor()
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			textName := fd.TextName()
			if textName == part {
				kind := fd.Kind()
				if kind == protoreflect.MessageKind {

					// Get message
					m := msg.ProtoReflect().Get(fd).Message()
					// Get field value from message
					nestedPath := strings.Join(pathParts[j+1:], ".")
					return GetFieldParentMessage(m.Interface(), nestedPath)
				}
			}
		}
	}
	return nil, "", status.Errorf(codes.Internal, "field not found")
}

func IsAlphanumericOrUnderscore(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_') {
			return false
		}
	}
	return true
}
