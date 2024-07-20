package validator

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type boolean struct {
	paths       []string
	getValue    func(v *Validator, msg protoreflect.ProtoMessage) bool
	description string
	v           *Validator
}

func (b *boolean) fieldPaths() []string {
	return b.paths
}

func (b *boolean) getDescription() string {
	return b.description
}

func (b *boolean) getValidator() *Validator {
	return b.v
}

func (b *boolean) setValidator(v *Validator) {
	b.getValue(v, v.protoMsg)
	b.v = v
}

func Bool(value bool) *boolean {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) bool {
		return value
	}
	return &boolean{description: fmt.Sprintf("%t", value), getValue: getValueFunc}
}

func BoolField(path string) *boolean {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) bool {
		return v.getBool(msg, path)
	}
	return &boolean{description: path, getValue: getValueFunc, paths: []string{path}}
}

func (b *boolean) Equals(b2 *boolean) *Rule {
	id := fmt.Sprintf("b-eq(%s,%s)", b.getDescription(), b2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be equal to %s", b.getDescription(), b2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be equal to %s", b.getDescription(), b2.getDescription()),
		condition:    fmt.Sprintf("%s equals %s", b.getDescription(), b2.getDescription()),
		notCondition: fmt.Sprintf("%s does not equal %s", b.getDescription(), b2.getDescription()),
	}
	args := []argI{b, b2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := b.getValue(b.v, msg)
		val2 := b2.getValue(b2.v, msg)
		return val1 != val2, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}
