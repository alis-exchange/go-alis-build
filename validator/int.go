package validator

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type integer struct {
	paths       []string
	getValue    func(v *Validator, msg protoreflect.ProtoMessage) int64
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
	i.getValue(v, v.protoMsg)
	i.v = v
}

func Int(value int64) *integer {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) int64 {
		return value
	}
	i := &integer{description: fmt.Sprintf("%d", value), getValue: getValueFunc}
	return i
}

func IntField(path string) *integer {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) int64 {
		return v.getInt(msg, path)
	}
	i := &integer{description: path, getValue: getValueFunc, paths: []string{path}}
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
		val1 := i.getValue(i.v, msg)
		val2 := i2.getValue(i2.v, msg)
		return val1 != val2, nil
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
		val1 := i.getValue(i.v, msg)
		val2 := i2.getValue(i2.v, msg)
		return val1 <= val2, nil
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
		val1 := i.getValue(i.v, msg)
		val2 := i2.getValue(i2.v, msg)
		return val1 < val2, nil
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
		val1 := i.getValue(i.v, msg)
		val2 := i2.getValue(i2.v, msg)
		return val1 >= val2, nil
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
		val1 := i.getValue(i.v, msg)
		val2 := i2.getValue(i2.v, msg)
		return val1 > val2, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (i *integer) InRange(min *integer, max *integer) *Rule {
	id := fmt.Sprintf("i-ir(%s,%s,%s)", i.getDescription(), min.getDescription(), max.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be in range %s to %s", i.getDescription(), min.getDescription(), max.getDescription()),
		notRule:      fmt.Sprintf("%s must not be in range %s to %s", i.getDescription(), min.getDescription(), max.getDescription()),
		condition:    fmt.Sprintf("%s is in range %s to %s", i.getDescription(), min.getDescription(), max.getDescription()),
		notCondition: fmt.Sprintf("%s is not in range %s to %s", i.getDescription(), min.getDescription(), max.getDescription()),
	}
	args := []argI{i, min, max}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val := i.getValue(i.v, msg)
		minVal := min.getValue(min.v, msg)
		maxVal := max.getValue(max.v, msg)
		return val < minVal || val > maxVal, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (i *integer) Plus(i2 *integer) *integer {
	description := fmt.Sprintf("(%s + %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newI := &integer{description: description, paths: newPaths}
	newI.getValue = func(v *Validator, msg protoreflect.ProtoMessage) int64 {
		return i.getValue(v, msg) + i2.getValue(v, msg)
	}
	return newI
}

func (i *integer) Minus(i2 *integer) *integer {
	description := fmt.Sprintf("(%s - %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newI := &integer{description: description, paths: newPaths}
	newI.getValue = func(v *Validator, msg protoreflect.ProtoMessage) int64 {
		return i.getValue(v, msg) - i2.getValue(v, msg)
	}
	return newI
}

func (i *integer) Times(i2 *integer) *integer {
	description := fmt.Sprintf("(%s * %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newI := &integer{description: description, paths: newPaths}
	newI.getValue = func(v *Validator, msg protoreflect.ProtoMessage) int64 {
		return i.getValue(v, msg) * i2.getValue(v, msg)
	}
	return newI
}

func (i *integer) DividedBy(i2 *integer) *integer {
	description := fmt.Sprintf("(%s / %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newI := &integer{description: description, paths: newPaths}
	newI.getValue = func(v *Validator, msg protoreflect.ProtoMessage) int64 {
		return i.getValue(v, msg) / i2.getValue(v, msg)
	}
	return newI
}

func (i *integer) Mod(i2 *integer) *integer {
	description := fmt.Sprintf("(%s %% %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newI := &integer{description: description, paths: newPaths}
	newI.getValue = func(v *Validator, msg protoreflect.ProtoMessage) int64 {
		return i.getValue(v, msg) % i2.getValue(v, msg)
	}
	return newI
}

func IntSum(integers ...*integer) *integer {
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
	newI.getValue = func(v *Validator, msg protoreflect.ProtoMessage) int64 {
		sum := int64(0)
		for _, i := range integers {
			sum += i.getValue(v, msg)
		}
		return sum
	}
	return newI
}
