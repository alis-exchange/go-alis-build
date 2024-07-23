package validator

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Returns the length of a boolean list
func BoolListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getBoolList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of a boolean list as a float
func BoolListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getBoolList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of a string list
func StringListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getStringList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of a string list as a float
func StringListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getStringList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of an integer list
func IntListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getIntList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of an integer list as a float
func IntListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getIntList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of a float list
func FloatListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getFloatList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of a float list as a float
func FloatListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getFloatList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of an enum list
func EnumListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getEnumList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of an enum list as a float
func EnumListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getEnumList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of a submessage list
func SubMsgListLength(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(v.getSubMessageList(msg, path)))}
	}
	return &integer{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}

// Returns the length of a submessage list as a float
func SubMsgListLengthAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(v.getSubMessageList(msg, path)))}
	}
	return &float{description: fmt.Sprintf("length of %s", path), getValues: getValuesFunc, paths: []string{path}}
}
