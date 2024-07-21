package validator

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func BoolListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getBoolList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func BoolListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getBoolList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func StringListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getStringList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func StringListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getStringList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func IntListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getIntList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func IntListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getIntList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func FloatListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getFloatList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func FloatListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getFloatList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func EnumListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getEnumList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func EnumListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getEnumList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func SubMsgListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getSubMessageList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

func SubMsgListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getSubMessageList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}
