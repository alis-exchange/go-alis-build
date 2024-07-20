package validator

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type float struct {
	paths       []string
	getValue    func(v *Validator, msg protoreflect.ProtoMessage) float64
	description string
	v           *Validator
}

func (f *float) fieldPaths() []string {
	return f.paths
}

func (f *float) getDescription() string {
	return f.description
}

func (f *float) getValidator() *Validator {
	return f.v
}

func (f *float) setValidator(v *Validator) {
	f.getValue(v, v.protoMsg)
	f.v = v
}

func Float(value float64) *float {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return value
	}
	n := &float{description: fmt.Sprintf("%f", value), getValue: getValueFunc}
	return n
}

func IntFieldAsFloat(path string) *float {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return float64(v.getInt(msg, path))
	}
	f := &float{description: path, getValue: getValueFunc, paths: []string{path}}
	return f
}

func FloatField(path string) *float {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return v.getFloat(msg, path)
	}
	f := &float{description: path, getValue: getValueFunc, paths: []string{path}}
	return f
}

func (f *float) Equals(f2 *float) *Rule {
	id := fmt.Sprintf("f-eq(%s,%s)", f.getDescription(), f2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be equal to %s", f.getDescription(), f2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be equal to %s", f.getDescription(), f2.getDescription()),
		condition:    fmt.Sprintf("%s equals %s", f.getDescription(), f2.getDescription()),
		notCondition: fmt.Sprintf("%s does not equal %s", f.getDescription(), f2.getDescription()),
	}
	args := []argI{f, f2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := f.getValue(f.v, msg)
		val2 := f2.getValue(f2.v, msg)
		return val1 != val2, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (f *float) GT(f2 *float) *Rule {
	id := fmt.Sprintf("f-gt(%s,%s)", f.getDescription(), f2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be greater than %s", f.getDescription(), f2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be greater than %s", f.getDescription(), f2.getDescription()),
		condition:    fmt.Sprintf("%s is greater than %s", f.getDescription(), f2.getDescription()),
		notCondition: fmt.Sprintf("%s is not greater than %s", f.getDescription(), f2.getDescription()),
	}
	args := []argI{f, f2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := f.getValue(f.v, msg)
		val2 := f2.getValue(f2.v, msg)
		return val1 <= val2, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (f *float) GTE(f2 *float) *Rule {
	id := fmt.Sprintf("f-gte(%s,%s)", f.getDescription(), f2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be greater than or equal to %s", f.getDescription(), f2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be greater than or equal to %s", f.getDescription(), f2.getDescription()),
		condition:    fmt.Sprintf("%s is greater than or equal to %s", f.getDescription(), f2.getDescription()),
		notCondition: fmt.Sprintf("%s is not greater than or equal to %s", f.getDescription(), f2.getDescription()),
	}
	args := []argI{f, f2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := f.getValue(f.v, msg)
		val2 := f2.getValue(f2.v, msg)
		return val1 < val2, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (f *float) LT(f2 *float) *Rule {
	id := fmt.Sprintf("f-lt(%s,%s)", f.getDescription(), f2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be less than %s", f.getDescription(), f2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be less than %s", f.getDescription(), f2.getDescription()),
		condition:    fmt.Sprintf("%s is less than %s", f.getDescription(), f2.getDescription()),
		notCondition: fmt.Sprintf("%s is not less than %s", f.getDescription(), f2.getDescription()),
	}
	args := []argI{f, f2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := f.getValue(f.v, msg)
		val2 := f2.getValue(f2.v, msg)
		return val1 >= val2, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (f *float) LTE(f2 *float) *Rule {
	id := fmt.Sprintf("f-lte(%s,%s)", f.getDescription(), f2.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be less than or equal to %s", f.getDescription(), f2.getDescription()),
		notRule:      fmt.Sprintf("%s must not be less than or equal to %s", f.getDescription(), f2.getDescription()),
		condition:    fmt.Sprintf("%s is less than or equal to %s", f.getDescription(), f2.getDescription()),
		notCondition: fmt.Sprintf("%s is not less than or equal to %s", f.getDescription(), f2.getDescription()),
	}
	args := []argI{f, f2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := f.getValue(f.v, msg)
		val2 := f2.getValue(f2.v, msg)
		return val1 > val2, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (f *float) InRange(min *float, max *float) *Rule {
	id := fmt.Sprintf("f-ir(%s,%s,%s)", f.getDescription(), min.getDescription(), max.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be in range %s to %s", f.getDescription(), min.getDescription(), max.getDescription()),
		notRule:      fmt.Sprintf("%s must not be in range %s to %s", f.getDescription(), min.getDescription(), max.getDescription()),
		condition:    fmt.Sprintf("%s is in range %s to %s", f.getDescription(), min.getDescription(), max.getDescription()),
		notCondition: fmt.Sprintf("%s is not in range %s to %s", f.getDescription(), min.getDescription(), max.getDescription()),
	}
	args := []argI{f, min, max}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val := f.getValue(f.v, msg)
		minVal := min.getValue(min.v, msg)
		maxVal := max.getValue(max.v, msg)
		return val < minVal || val > maxVal, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (f *float) Plus(f2 *float) *float {
	description := fmt.Sprintf("(%s + %s)", f.getDescription(), f2.getDescription())
	newPaths := append(f.paths, f2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return f.getValue(v, msg) + f2.getValue(v, msg)
	}
	return newF
}

func (f *float) Minus(f2 *float) *float {
	description := fmt.Sprintf("(%s - %s)", f.getDescription(), f2.getDescription())
	newPaths := append(f.paths, f2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return f.getValue(v, msg) - f2.getValue(v, msg)
	}
	return newF
}

func (f *float) Times(f2 *float) *float {
	description := fmt.Sprintf("(%s * %s)", f.getDescription(), f2.getDescription())
	newPaths := append(f.paths, f2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return f.getValue(v, msg) * f2.getValue(v, msg)
	}
	return newF
}

func (f *float) DividedBy(f2 *float) *float {
	description := fmt.Sprintf("(%s / %s)", f.getDescription(), f2.getDescription())
	newPaths := append(f.paths, f2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return f.getValue(v, msg) / f2.getValue(v, msg)
	}
	return newF
}

func (f *float) Mod(f2 *float) *float {
	description := fmt.Sprintf("(%s %% %s)", f.getDescription(), f2.getDescription())
	newPaths := append(f.paths, f2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return float64(int64(f.getValue(v, msg)) % int64(f2.getValue(v, msg)))
	}
	return newF
}

func Sum(floats ...*float) *float {
	description := fmt.Sprintf("sum of %v", floats)
	newF := &float{description: description}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		sum := 0.0
		for _, f := range floats {
			sum += f.getValue(v, msg)
		}
		return sum
	}
	return newF
}
