package validator

import (
	"context"
	"fmt"
	"strings"

	"go.alis.build/alog"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type integer struct {
	repeated    bool
	paths       []string
	getValues   func(v *Validator, msg protoreflect.ProtoMessage) []int64
	description string
	v           *Validator
}

func (i *integer) fieldPaths() []string {
	return i.paths
}

func (i *integer) getDescription() string {
	return i.description
}

func (i *integer) getValidator() *Validator {
	return i.v
}

func (i *integer) setValidator(v *Validator) {
	i.getValues(v, v.protoMsg)
	i.v = v
}

func Int(value int64) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{value}
	}
	i := &integer{description: fmt.Sprintf("%d", value), getValues: getValuesFunc}
	return i
}

func IntField(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{v.getInt(msg, path)}
	}
	i := &integer{description: path, getValues: getValuesFunc, paths: []string{path}}
	return i
}

func EachIntIn(path string) *integer {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return v.getIntList(msg, path)
	}
	i := &integer{description: fmt.Sprintf("each int in %s", path), getValues: getValuesFunc, paths: []string{path}, repeated: true}
	return i
}

func (i *integer) Equals(i2 *integer) *Rule {
	id := fmt.Sprintf("i-eq(%s,%s)", i.getDescription(), i2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be equal to %s", i.getDescription(), i2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be equal to %s", i.getDescription(), i2.getDescription()),
		condition:    fmt.Sprintf("%s equals %s", i.getDescription(), i2.getDescription()),
		notCondition: fmt.Sprintf("%s does not equal %s", i.getDescription(), i2.getDescription()),
	}
	args := []argI{i, i2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		for _, val1 := range i.getValues(i.v, msg) {
			for _, val2 := range i2.getValues(i2.v, msg) {
				if val1 != val2 {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (i *integer) GT(i2 *integer) *Rule {
	id := fmt.Sprintf("i-gt(%s,%s)", i.getDescription(), i2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be greater than %s", i.getDescription(), i2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be greater than %s", i.getDescription(), i2.getDescription()),
		condition:    fmt.Sprintf("%s is greater than %s", i.getDescription(), i2.getDescription()),
		notCondition: fmt.Sprintf("%s is not greater than %s", i.getDescription(), i2.getDescription()),
	}
	args := []argI{i, i2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		for _, val1 := range i.getValues(i.v, msg) {
			for _, val2 := range i2.getValues(i2.v, msg) {
				if val1 <= val2 {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (i *integer) GTE(i2 *integer) *Rule {
	id := fmt.Sprintf("i-gte(%s,%s)", i.getDescription(), i2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be greater than or equal to %s", i.getDescription(), i2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be greater than or equal to %s", i.getDescription(), i2.getDescription()),
		condition:    fmt.Sprintf("%s is greater than or equal to %s", i.getDescription(), i2.getDescription()),
		notCondition: fmt.Sprintf("%s is not greater than or equal to %s", i.getDescription(), i2.getDescription()),
	}
	args := []argI{i, i2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		for _, val1 := range i.getValues(i.v, msg) {
			for _, val2 := range i2.getValues(i2.v, msg) {
				if val1 < val2 {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (i *integer) LT(i2 *integer) *Rule {
	id := fmt.Sprintf("i-lt(%s,%s)", i.getDescription(), i2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be less than %s", i.getDescription(), i2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be less than %s", i.getDescription(), i2.getDescription()),
		condition:    fmt.Sprintf("%s is less than %s", i.getDescription(), i2.getDescription()),
		notCondition: fmt.Sprintf("%s is not less than %s", i.getDescription(), i2.getDescription()),
	}
	args := []argI{i, i2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		for _, val1 := range i.getValues(i.v, msg) {
			for _, val2 := range i2.getValues(i2.v, msg) {
				if val1 >= val2 {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (i *integer) LTE(i2 *integer) *Rule {
	id := fmt.Sprintf("i-lte(%s,%s)", i.getDescription(), i2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be less than or equal to %s", i.getDescription(), i2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be less than or equal to %s", i.getDescription(), i2.getDescription()),
		condition:    fmt.Sprintf("%s is less than or equal to %s", i.getDescription(), i2.getDescription()),
		notCondition: fmt.Sprintf("%s is not less than or equal to %s", i.getDescription(), i2.getDescription()),
	}
	args := []argI{i, i2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		for _, val1 := range i.getValues(i.v, msg) {
			for _, val2 := range i2.getValues(i2.v, msg) {
				if val1 > val2 {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (i *integer) InRange(min *integer, max *integer) *Rule {
	if min.repeated || max.repeated {
		alog.Fatalf(context.Background(), "min and max values for InRange must be single values")
	}
	id := fmt.Sprintf("i-ir(%s,%s,%s)", i.getDescription(), min.getDescription(), max.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be in range %s to %s", i.getDescription(), min.getDescription(), max.getDescription()),
		notRule:      fmt.Sprintf("%s must not be in range %s to %s", i.getDescription(), min.getDescription(), max.getDescription()),
		condition:    fmt.Sprintf("%s is in range %s to %s", i.getDescription(), min.getDescription(), max.getDescription()),
		notCondition: fmt.Sprintf("%s is not in range %s to %s", i.getDescription(), min.getDescription(), max.getDescription()),
	}
	args := []argI{i, min, max}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		for _, val := range i.getValues(i.v, msg) {
			if val < min.getValues(min.v, msg)[0] || val > max.getValues(max.v, msg)[0] {
				return true, nil
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (i *integer) Plus(i2 *integer) *integer {
	if i.repeated || i2.repeated {
		alog.Fatalf(context.Background(), "Plus() is not supported for repeated fields")
	}
	description := fmt.Sprintf("(%s + %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newI := &integer{description: description, paths: newPaths}
	newI.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{i.getValues(v, msg)[0] + i2.getValues(v, msg)[0]}
	}
	return newI
}

func (i *integer) Minus(i2 *integer) *integer {
	if i.repeated || i2.repeated {
		alog.Fatalf(context.Background(), "Minus() is not supported for repeated fields")
	}
	description := fmt.Sprintf("(%s - %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newI := &integer{description: description, paths: newPaths}
	newI.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{i.getValues(v, msg)[0] - i2.getValues(v, msg)[0]}
	}
	return newI
}

func (i *integer) Times(i2 *integer) *integer {
	if i.repeated || i2.repeated {
		alog.Fatalf(context.Background(), "Times() is not supported for repeated fields")
	}
	description := fmt.Sprintf("(%s * %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newI := &integer{description: description, paths: newPaths}
	newI.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{i.getValues(v, msg)[0] * i2.getValues(v, msg)[0]}
	}
	return newI
}

func (i *integer) DividedBy(i2 *integer) *integer {
	if i.repeated || i2.repeated {
		alog.Fatalf(context.Background(), "DividedBy() is not supported for repeated fields")
	}
	description := fmt.Sprintf("(%s / %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newI := &integer{description: description, paths: newPaths}
	newI.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{i.getValues(v, msg)[0] / i2.getValues(v, msg)[0]}
	}
	return newI
}

func (i *integer) Mod(i2 *integer) *integer {
	if i.repeated || i2.repeated {
		alog.Fatalf(context.Background(), "Mod() is not supported for repeated fields")
	}
	description := fmt.Sprintf("(%s %% %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newI := &integer{description: description, paths: newPaths}
	newI.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{i.getValues(v, msg)[0] % i2.getValues(v, msg)[0]}
	}
	return newI
}

func IntSum(integers ...*integer) *integer {
	for _, i := range integers {
		if i.repeated {
			alog.Fatalf(context.Background(), "IntSum() is not supported for repeated fields")
		}
	}
	descriptions := make([]string, len(integers))
	for i, intgr := range integers {
		descriptions[i] = intgr.getDescription()
	}
	description := fmt.Sprintf("sum of %s", strings.Join(descriptions, " and "))
	newPaths := []string{}
	for _, i := range integers {
		newPaths = append(newPaths, i.paths...)
	}
	newI := &integer{description: description, paths: newPaths}
	newI.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		sum := int64(0)
		for _, i := range integers {
			sum += i.getValues(v, msg)[0]
		}
		return []int64{sum}
	}
	return newI
}
