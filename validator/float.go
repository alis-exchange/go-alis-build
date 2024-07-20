package validator

import (
	"context"
	"fmt"

	"go.alis.build/alog"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type float struct {
	repeated    bool
	paths       []string
	getValues   func(v *Validator, msg protoreflect.ProtoMessage) []float64
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
	f.getValues(v, v.protoMsg)
	f.v = v
}

func Float(value float64) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{value}
	}
	n := &float{description: fmt.Sprintf("%f", value), getValues: getValuesFunc}
	return n
}

func IntFieldAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(v.getInt(msg, path))}
	}
	f := &float{description: path, getValues: getValuesFunc, paths: []string{path}}
	return f
}

func EachIntInAsFloat(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		ints := v.getIntList(msg, path)
		floats := make([]float64, len(ints))
		for i, val := range ints {
			floats[i] = float64(val)
		}
		return floats
	}
	f := &float{description: fmt.Sprintf("each integer in %s", path), getValues: getValuesFunc, paths: []string{path}, repeated: true}
	return f
}

func EachFloatIn(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return v.getFloatList(msg, path)
	}
	f := &float{description: fmt.Sprintf("each float in %s", path), getValues: getValuesFunc, paths: []string{path}, repeated: true}
	return f
}

func FloatField(path string) *float {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{v.getFloat(msg, path)}
	}
	f := &float{description: path, getValues: getValuesFunc, paths: []string{path}}
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
		for _, val1 := range f.getValues(f.v, msg) {
			for _, val2 := range f2.getValues(f2.v, msg) {
				if val1 != val2 {
					return true, nil
				}
			}
		}
		return false, nil
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
		for _, val1 := range f.getValues(f.v, msg) {
			for _, val2 := range f2.getValues(f2.v, msg) {
				if val1 <= val2 {
					return true, nil
				}
			}
		}
		return false, nil
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
		for _, val1 := range f.getValues(f.v, msg) {
			for _, val2 := range f2.getValues(f2.v, msg) {
				if val1 < val2 {
					return true, nil
				}
			}
		}
		return false, nil
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
		for _, val1 := range f.getValues(f.v, msg) {
			for _, val2 := range f2.getValues(f2.v, msg) {
				if val1 >= val2 {
					return true, nil
				}
			}
		}
		return false, nil
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
		for _, val1 := range f.getValues(f.v, msg) {
			for _, val2 := range f2.getValues(f2.v, msg) {
				if val1 > val2 {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (f *float) InRange(min *float, max *float) *Rule {
	if min.repeated || max.repeated {
		alog.Fatalf(context.Background(), "min and max must not be repeated")
	}
	id := fmt.Sprintf("f-ir(%s,%s,%s)", f.getDescription(), min.getDescription(), max.getDescription())
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be in range %s to %s", f.getDescription(), min.getDescription(), max.getDescription()),
		notRule:      fmt.Sprintf("%s must not be in range %s to %s", f.getDescription(), min.getDescription(), max.getDescription()),
		condition:    fmt.Sprintf("%s is in range %s to %s", f.getDescription(), min.getDescription(), max.getDescription()),
		notCondition: fmt.Sprintf("%s is not in range %s to %s", f.getDescription(), min.getDescription(), max.getDescription()),
	}
	args := []argI{f, min, max}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		for _, val := range f.getValues(f.v, msg) {
			if val < min.getValues(min.v, msg)[0] || val > max.getValues(max.v, msg)[0] {
				return true, nil
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (f *float) Plus(f2 *float) *float {
	if f.repeated || f2.repeated {
		alog.Fatalf(context.Background(), "Plus() is not supported for repeated fields")
	}
	description := fmt.Sprintf("(%s + %s)", f.getDescription(), f2.getDescription())
	newPaths := append(f.paths, f2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{f.getValues(v, msg)[0] + f2.getValues(v, msg)[0]}
	}
	return newF
}

func (f *float) Minus(f2 *float) *float {
	description := fmt.Sprintf("(%s - %s)", f.getDescription(), f2.getDescription())
	newPaths := append(f.paths, f2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{f.getValues(v, msg)[0] - f2.getValues(v, msg)[0]}
	}
	return newF
}

func (f *float) Times(f2 *float) *float {
	description := fmt.Sprintf("(%s * %s)", f.getDescription(), f2.getDescription())
	newPaths := append(f.paths, f2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{f.getValues(v, msg)[0] * f2.getValues(v, msg)[0]}
	}
	return newF
}

func (f *float) DividedBy(f2 *float) *float {
	description := fmt.Sprintf("(%s / %s)", f.getDescription(), f2.getDescription())
	newPaths := append(f.paths, f2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{f.getValues(v, msg)[0] / f2.getValues(v, msg)[0]}
	}
	return newF
}

func (f *float) Mod(f2 *float) *float {
	description := fmt.Sprintf("(%s %% %s)", f.getDescription(), f2.getDescription())
	newPaths := append(f.paths, f2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(int(f.getValues(v, msg)[0]) % int(f2.getValues(v, msg)[0]))}
	}
	return newF
}

func Sum(floats ...*float) *float {
	description := fmt.Sprintf("sum of %v", floats)
	newF := &float{description: description}
	newF.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		sum := 0.0
		for _, f := range floats {
			sum += f.getValues(v, msg)[0]
		}
		return []float64{sum}
	}
	return newF
}
