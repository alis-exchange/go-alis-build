package validator

import (
	"context"
	"strings"

	"go.alis.build/alog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (v *Validator) getFieldDescriptor(msg protoreflect.ProtoMessage, path string) (protoreflect.FieldDescriptor, error) {
	md := msg.ProtoReflect().Descriptor()
	index := 0
	if val, ok := v.fieldIndex[path]; ok {
		index = val
	} else {
		foundIndex := false
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			if fd.TextName() == path {
				v.fieldIndex[path] = i
				index = i
				foundIndex = true
			}
		}
		if !foundIndex {
			return nil, status.Errorf(codes.Internal, "%s not found", path)
		}
	}

	return md.Fields().Get(index), nil
}

func (v *Validator) getValueWithReflection(msg protoreflect.ProtoMessage, path string, allowedKinds []protoreflect.Kind) protoreflect.Value {
	// not supporting reflection for nested fields
	if strings.Contains(path, ".") {
		alog.Fatalf(context.Background(), "nested fields not supported for reflection. Please setup the appropriate getter function for %s", path)
	}
	v.issueWarning("using reflection is very slow. Please setup the appropriate getter function for %s", path)

	fd, err := v.getFieldDescriptor(msg, path)
	if err != nil {
		alog.Fatalf(context.Background(), "error getting field descriptor for %s: %s", path, err)
	}

	if len(allowedKinds) != 0 {
		foundKind := false
		kindStrings := make([]string, len(allowedKinds))
		for _, kind := range allowedKinds {
			if fd.Kind() == kind {
				foundKind = true
				break
			}
			kindStrings = append(kindStrings, kind.String())
		}
		if !foundKind {
			alog.Fatalf(context.Background(), "%s is not a valid %s", path, strings.Join(kindStrings, " or "))
		}
	}

	return msg.ProtoReflect().Get(fd)
}

func (v *Validator) getString(msg protoreflect.ProtoMessage, path string) string {
	if v.StringGetter != nil {
		val, err := v.StringGetter(msg, path)
		if err != nil {
			v.issueWarning("string getter failed to get value for %s: %v", path, err)
		} else {
			return val
		}
	} else {
		v.issueWarning("no string getter function defined")
	}

	return v.getValueWithReflection(msg, path, []protoreflect.Kind{protoreflect.StringKind}).String()
}

func (v *Validator) getInt(msg protoreflect.ProtoMessage, path string) int64 {
	if v.IntGetter != nil {
		val, err := v.IntGetter(msg, path)
		if err != nil {
			v.issueWarning("error getting int value from %s: %v", path, err)
		} else {
			return val
		}
	} else {
		v.issueWarning("no int getter function defined")
	}
	return v.getValueWithReflection(msg, path, []protoreflect.Kind{protoreflect.Int32Kind, protoreflect.Int64Kind}).Int()
}

func (v *Validator) getFloat(msg protoreflect.ProtoMessage, path string) float64 {
	if v.FloatGetter != nil {
		val, err := v.FloatGetter(msg, path)
		if err != nil {
			v.issueWarning("error getting float value from %s: %v", path, err)
		} else {
			return val
		}
	} else {
		v.issueWarning("no float getter function defined")
	}
	return v.getValueWithReflection(msg, path, []protoreflect.Kind{protoreflect.FloatKind, protoreflect.DoubleKind}).Float()
}

func (v *Validator) getBool(msg protoreflect.ProtoMessage, path string) bool {
	if v.BoolGetter != nil {
		val, err := v.BoolGetter(msg, path)
		if err != nil {
			v.issueWarning("error getting bool value from %s: %v", path, err)
		} else {
			return val
		}
	} else {
		v.issueWarning("no bool getter function defined")
	}
	return v.getValueWithReflection(msg, path, []protoreflect.Kind{protoreflect.BoolKind}).Bool()
}

func (v *Validator) getSubMessage(msg protoreflect.ProtoMessage, path string) protoreflect.ProtoMessage {
	if v.SubMessageGetter != nil {
		val, err := v.SubMessageGetter(msg, path)
		if err != nil {
			v.issueWarning("error getting sub message from %s: %v", path, err)
		} else {
			return val
		}
	} else {
		v.issueWarning("no sub message getter function defined")
	}
	return v.getValueWithReflection(msg, path, []protoreflect.Kind{protoreflect.MessageKind}).Message().Interface()
}

func (v *Validator) getStringList(msg protoreflect.ProtoMessage, path string) []string {
	if v.StringListGetter != nil {
		val, err := v.StringListGetter(msg, path)
		if err != nil {
			v.issueWarning("error getting string list value from %s: %v", path, err)
		} else {
			return val
		}
	} else {
		v.issueWarning("no string list getter function defined")
	}
	return v.getStringListWithReflection(msg, path)
}

func (v *Validator) getStringListWithReflection(msg protoreflect.ProtoMessage, path string) []string {
	values := v.getValueWithReflection(msg, path, []protoreflect.Kind{protoreflect.StringKind}).List()
	strList := make([]string, values.Len())
	for i := 0; i < values.Len(); i++ {
		strList[i] = values.Get(i).String()
	}
	return strList
}

func (v *Validator) getIntList(msg protoreflect.ProtoMessage, path string) []int64 {
	if v.IntListGetter != nil {
		val, err := v.IntListGetter(msg, path)
		if err != nil {
			v.issueWarning("error getting int list value from %s: %v", path, err)
		} else {
			return val
		}
	} else {
		v.issueWarning("no int list getter function defined")
	}
	return v.getIntListWithReflection(msg, path)
}

func (v *Validator) getIntListWithReflection(msg protoreflect.ProtoMessage, path string) []int64 {
	values := v.getValueWithReflection(msg, path, []protoreflect.Kind{protoreflect.Int32Kind, protoreflect.Int64Kind}).List()
	intList := make([]int64, values.Len())
	for i := 0; i < values.Len(); i++ {
		intList[i] = values.Get(i).Int()
	}
	return intList
}

func (v *Validator) getFloatList(msg protoreflect.ProtoMessage, path string) []float64 {
	if v.FloatListGetter != nil {
		val, err := v.FloatListGetter(msg, path)
		if err != nil {
			v.issueWarning("error getting float list value from %s: %v", path, err)
		} else {
			return val
		}
	} else {
		v.issueWarning("no float list getter function defined")
	}
	return v.getFloatListWithReflection(msg, path)
}

func (v *Validator) getFloatListWithReflection(msg protoreflect.ProtoMessage, path string) []float64 {
	values := v.getValueWithReflection(msg, path, []protoreflect.Kind{protoreflect.FloatKind, protoreflect.DoubleKind}).List()
	floatList := make([]float64, values.Len())
	for i := 0; i < values.Len(); i++ {
		floatList[i] = values.Get(i).Float()
	}
	return floatList
}

func (v *Validator) getBoolList(msg protoreflect.ProtoMessage, path string) []bool {
	if v.BoolListGetter != nil {
		val, err := v.BoolListGetter(msg, path)
		if err != nil {
			v.issueWarning("error getting bool list value from %s: %v", path, err)
		} else {
			return val
		}
	} else {
		v.issueWarning("no bool list getter function defined")
	}
	return v.getBoolListWithReflection(msg, path)
}

func (v *Validator) getBoolListWithReflection(msg protoreflect.ProtoMessage, path string) []bool {
	values := v.getValueWithReflection(msg, path, []protoreflect.Kind{protoreflect.BoolKind}).List()
	boolList := make([]bool, values.Len())
	for i := 0; i < values.Len(); i++ {
		boolList[i] = values.Get(i).Bool()
	}
	return boolList
}

func (v *Validator) getSubMessageList(msg protoreflect.ProtoMessage, path string) []protoreflect.ProtoMessage {
	if v.SubMessageListGetter != nil {
		val, err := v.SubMessageListGetter(msg, path)
		if err != nil {
			v.issueWarning("error getting sub message list value from %s: %v", path, err)
		} else {
			return val
		}
	} else {
		v.issueWarning("no sub message list getter function defined")
	}
	return v.getSubMessageListWithReflection(msg, path)
}

func (v *Validator) getSubMessageListWithReflection(msg protoreflect.ProtoMessage, path string) []protoreflect.ProtoMessage {
	values := v.getValueWithReflection(msg, path, []protoreflect.Kind{protoreflect.MessageKind}).List()
	subMessageList := make([]protoreflect.ProtoMessage, values.Len())
	for i := 0; i < values.Len(); i++ {
		subMessageList[i] = values.Get(i).Message().Interface()
	}
	return subMessageList
}
