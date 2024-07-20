package validator

import (
	"context"

	"go.alis.build/alog"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type enum struct {
	expectedEnumType string
	paths            []string
	getValue         func(v *Validator, msg protoreflect.ProtoMessage) protoreflect.EnumNumber
	description      string
	v                *Validator
}

func (f *enum) fieldPaths() []string {
	return f.paths
}

func (f *enum) getDescription() string {
	return f.description
}

func (f *enum) getValidator() *Validator {
	return f.v
}

func (f *enum) setValidator(v *Validator) {
	if len(f.paths) > 0 {
		fd, err := v.getFieldDescriptor(v.protoMsg, f.paths[0])
		if err != nil {
			alog.Fatalf(context.Background(), "field descriptor not found for %s", f.paths[0])
		}
		if string(fd.Enum().FullName()) != f.expectedEnumType {
			alog.Fatalf(context.Background(), "expected enum type %s but got %s", f.expectedEnumType, fd.FullName())
		}
	}
	f.v = v
}

func Enum(value protoreflect.Enum) *enum {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) protoreflect.EnumNumber {
		return value.Number()
	}
	n := &enum{description: string(value.Descriptor().Values().ByNumber(value.Number()).Name()), getValue: getValueFunc, expectedEnumType: string(value.Descriptor().FullName())}
	return n
}

func EnumField(path string) *enum {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) protoreflect.EnumNumber {
		return v.getEnum(msg, path)
	}
	f := &enum{description: path, getValue: getValueFunc, paths: []string{path}}
	return f
}

func (f *enum) Equals(f2 *enum) *Rule {
	id := "e-eq(" + f.getDescription() + "," + f2.getDescription() + ")"
	descr := &Descriptions{
		rule:         f.getDescription() + " must be set to " + f2.getDescription(),
		notRule:      f.getDescription() + " must not be set to " + f2.getDescription(),
		condition:    f.getDescription() + " is set to " + f2.getDescription(),
		notCondition: f.getDescription() + " is not set to " + f2.getDescription(),
	}
	if f.expectedEnumType == "" {
		f.expectedEnumType = f2.expectedEnumType
	}
	if f2.expectedEnumType == "" {
		f2.expectedEnumType = f.expectedEnumType
	}
	args := []argI{f, f2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		return f.getValue(f.v, msg) != f2.getValue(f2.v, msg), nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}
