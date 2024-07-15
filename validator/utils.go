package validator

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func getMsgType(msg protoreflect.ProtoMessage) string {
	return string(msg.ProtoReflect().Descriptor().FullName())
}

func getStringField(msg protoreflect.ProtoMessage, fieldName string) (string, error) {
	parentMsg, fieldName, err := getFieldParentMessage(msg, fieldName)
	if err != nil {
		return "", err
	}
	md := parentMsg.ProtoReflect().Descriptor()
	var value string

	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if fd.TextName() == fieldName {
			if fd.Kind() == protoreflect.StringKind {
				value = parentMsg.ProtoReflect().Get(fd).String()
				return value, nil
			} else {
				return "", status.Errorf(codes.InvalidArgument, "%s is not a string field", fieldName)
			}
		}
	}

	return "", status.Errorf(codes.Internal, "%s not found", fieldName)
}

func getFieldParentMessage(msg protoreflect.ProtoMessage, path string) (protoreflect.ProtoMessage, string, error) {
	pathParts := strings.Split(path, ".")
	if len(pathParts) == 1 {
		return msg, path, nil
	}
	// remove last part
	for j, part := range pathParts {
		// if part does not contain alphanumeric or underscore, return error
		if !isAlphanumericOrUnderscore(part) {
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
					return getFieldParentMessage(m.Interface(), nestedPath)
				}
			}
		}
	}
	return nil, "", status.Errorf(codes.Internal, "field not found")
}

func isAlphanumericOrUnderscore(s string) bool {
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
