package validator

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type boolean struct {
	val  bool
	path string
	v    *Validator
}

func BoolValue(value bool) *boolean {
	return &boolean{val: value}
}

func BoolField(path string) *boolean {
	return &boolean{path: path}
}

func (b *boolean) Boolean() bool {
	if b.path != "" {
		return b.v.getBool(b.v.protoMsg, b.path)
	} else {
		return b.val
	}
}

func (b *boolean) fieldPaths() []string {
	return []string{b.path}
}

func (b *boolean) setValidator(v *Validator) {
	b.v = v
}

func (b *boolean) getDescription() string {
	if b.path != "" {
		return b.path
	} else {
		return fmt.Sprintf("'%t'", b.val)
	}
}

func (b *boolean) Equals(b2 *boolean) *Rule {
	description := fmt.Sprintf("%s must be equal to %s", b.getDescription(), b2.getDescription())
	notDescription := fmt.Sprintf("%s must not be equal to %s", b.getDescription(), b2.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("b-eq(%s,%s)", b.getDescription(), b2.getDescription()),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := b.Boolean()
			val2 := b2.Boolean()
			return val1 != val2, nil
		},
		arguments: []argI{b, b2},
	})
}
