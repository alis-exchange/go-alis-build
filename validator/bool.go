package validator

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type boolean struct {
	paths       []string
	getValues   func(v *Validator, msg protoreflect.ProtoMessage) []bool
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
	b.getValues(v, v.protoMsg)
	b.v = v
}

func Bool(value bool) *boolean {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []bool {
		return []bool{value}
	}
	return &boolean{description: fmt.Sprintf("%t", value), getValues: getValuesFunc}
}

func BoolField(path string) *boolean {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []bool {
		return []bool{v.getBool(msg, path)}
	}
	return &boolean{description: path, getValues: getValuesFunc, paths: []string{path}}
}

func EachBoolIn(path string) *boolean {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []bool {
		return v.getBoolList(msg, path)
	}
	return &boolean{description: fmt.Sprintf("each bool in %s", path), getValues: getValuesFunc, paths: []string{path}}
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
		for _, bEl := range b.getValues(b.v, msg) {
			for _, b2El := range b2.getValues(b2.v, msg) {
				if bEl != b2El {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}
