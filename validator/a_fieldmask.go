package validator

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.alis.build/alog"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type fieldmask struct {
	paths       []string
	getValues   func(v *Validator, msg protoreflect.ProtoMessage) []*fieldmaskpb.FieldMask
	description string
	v           *Validator
}

func (f *fieldmask) fieldPaths() []string {
	return f.paths
}

func (f *fieldmask) getDescription() string {
	return f.description
}

func (f *fieldmask) getValidator() *Validator {
	return f.v
}

func (f *fieldmask) setValidator(v *Validator) {
	f.getValues(v, v.protoMsg)
	f.v = v
}

// google.protobuf.FieldMask field
func FieldMaskField(path string) *fieldmask {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []*fieldmaskpb.FieldMask {
		fm, ok := v.getSubMessage(msg, path).(*fieldmaskpb.FieldMask)
		if !ok {
			alog.Fatalf(context.Background(), "%s is not a FieldMask", path)
		}
		return []*fieldmaskpb.FieldMask{fm}
	}
	return &fieldmask{description: path, getValues: getValuesFunc, paths: []string{path}}
}

// Rule that checks ensures the fieldmask is not empty
func (f *fieldmask) NotEmpty() *Rule {
	id := "fm-pop"
	descr := &Descriptions{
		rule:         f.description + " cannot be empty",
		notRule:      f.description + " must be empty",
		condition:    f.description + " is not empty",
		notCondition: f.description + " is empty",
	}
	args := []argI{f}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		fm := f.getValues(f.getValidator(), msg)[0]
		if fm == nil {
			return true, nil
		}
		return len(fm.GetPaths()) == 0, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Rule that ensures the fieldmask contains only the allowed fields
func (f *fieldmask) OnlyContains(allowedFields []string) *Rule {
	id := "fm-oc"
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s can only contain %s", f.description, strings.Join(allowedFields, ", ")),
		notRule:      fmt.Sprintf("%s cannot only contain %s", f.description, strings.Join(allowedFields, ", ")),
		condition:    fmt.Sprintf("%s only contains %s", f.description, strings.Join(allowedFields, ", ")),
		notCondition: fmt.Sprintf("%s does not only contain %s", f.description, strings.Join(allowedFields, ", ")),
	}
	args := []argI{f}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		fm := f.getValues(f.getValidator(), msg)[0]
		if fm == nil {
			return false, nil
		}
		for _, path := range fm.GetPaths() {
			if !slices.Contains(allowedFields, path) {
				return true, nil
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Rule that ensures the fieldmask does not contain the disallowed fields
func (f *fieldmask) DoesNotContain(disallowedFields []string) *Rule {
	id := "fm-dnc"
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s cannot contain %s", f.description, strings.Join(disallowedFields, ", ")),
		notRule:      fmt.Sprintf("%s must contain %s", f.description, strings.Join(disallowedFields, ", ")),
		condition:    fmt.Sprintf("%s does not contain %s", f.description, strings.Join(disallowedFields, ", ")),
		notCondition: fmt.Sprintf("%s contains %s", f.description, strings.Join(disallowedFields, ", ")),
	}
	args := []argI{f}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		fm := f.getValues(f.getValidator(), msg)[0]
		if fm == nil {
			return false, nil
		}
		for _, path := range fm.GetPaths() {
			if slices.Contains(disallowedFields, path) {
				return true, nil
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}
