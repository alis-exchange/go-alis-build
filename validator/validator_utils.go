package validator

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (v *Validator) GetFieldInfo(data interface{}, fieldPath string, fieldInfoCache map[string]*FieldInfo) (*FieldInfo, error) {
	if fieldInfoCache != nil {
		if val, ok := fieldInfoCache[fieldPath]; ok {
			return val, nil
		}
	}
	msg := data.(protoreflect.ProtoMessage)
	parentMsg, fieldName, err := GetFieldParentMessage(msg, fieldPath)
	if err != nil {
		return nil, err
	}
	md := parentMsg.ProtoReflect().Descriptor()
	index := 0
	if val, ok := v.FieldIndex[fieldName]; ok {
		index = val
	} else {
		foundIndex := false
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			if fd.TextName() == fieldName {
				v.FieldIndex[fieldName] = i
				index = i
				foundIndex = true
			}
		}
		if !foundIndex {
			return nil, status.Errorf(codes.Internal, "%s not found", fieldName)
		}
	}

	// validate
	fd := md.Fields().Get(index)
	println(fd.HasPresence())
	if fd.TextName() != fieldName {
		return nil, status.Errorf(codes.Internal, "%s not found", fieldName)
	}
	fieldInfo := FieldInfo{
		Descriptor: fd,
		Value:      parentMsg.ProtoReflect().Get(fd),
	}
	if fieldInfoCache != nil {
		fieldInfoCache[fieldPath] = &fieldInfo
	}
	return &fieldInfo, nil
}

func (v *Validator) GetStringField(data interface{}, fieldPath string) (string, error) {
	fieldInfo, err := v.GetFieldInfo(data, fieldPath, nil)
	if err != nil {
		return "", err
	}
	if fieldInfo.Descriptor.Kind() != protoreflect.StringKind {
		return "", status.Errorf(codes.Internal, "%s is not a string field", fieldPath)
	}
	return fieldInfo.Value.String(), nil
}
