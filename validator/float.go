package validator

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type float struct {
	paths       []string
	getValue    func(v *Validator, msg protoreflect.ProtoMessage) float64
	v           *Validator
	description string
}

func Float(value float64) *float {
	n := &float{description: fmt.Sprintf("%f", value), getValue: func(v *Validator, msg protoreflect.ProtoMessage) float64 { return value }}
	return n
}

func IntFieldAsFloat(path string) *float {
	i := &float{paths: []string{path}, description: path, getValue: func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return float64(v.getInt(msg, path))
	}}
	return i
}

func FloatField(path string) *float {
	i := &float{paths: []string{path}, description: path, getValue: func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return v.getFloat(msg, path)
	}}
	return i
}

func (i *float) fieldPaths() []string {
	return i.paths
}

func (i *float) setValidator(v *Validator) {
	i.v = v
}

func (i *float) getDescription() string {
	return i.description
}

func (i *float) Equals(i2 *float) *Rule {
	description := fmt.Sprintf("%s must be equal to %s", i.getDescription(), i2.getDescription())
	notDescription := fmt.Sprintf("%s must not be equal to %s", i.getDescription(), i2.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("f-eq(%s,%s)", i.getDescription(), i2.getDescription()),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := i.getValue(i.v, msg)
			val2 := i2.getValue(i2.v, msg)
			return val1 != val2, nil
		},
		arguments: []argI{i, i2},
	})
}

func (i *float) GT(i2 *float) *Rule {
	description := fmt.Sprintf("%s must be greater than %s", i.getDescription(), i2.getDescription())
	notDescription := fmt.Sprintf("%s must not be greater than %s", i.getDescription(), i2.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("f-gt(%s,%s)", i.getDescription(), i2.getDescription()),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := i.getValue(i.v, msg)
			val2 := i2.getValue(i2.v, msg)
			return val1 <= val2, nil
		},
		arguments: []argI{i, i2},
	})
}

func (i *float) GTE(i2 *float) *Rule {
	description := fmt.Sprintf("%s must be greater than or equal to %s", i.getDescription(), i2.getDescription())
	notDescription := fmt.Sprintf("%s must not be greater than or equal to %s", i.getDescription(), i2.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("f-gte(%s,%s)", i.getDescription(), i2.getDescription()),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := i.getValue(i.v, msg)
			val2 := i2.getValue(i2.v, msg)
			return val1 < val2, nil
		},
		arguments: []argI{i, i2},
	})
}

func (i *float) LT(i2 *float) *Rule {
	description := fmt.Sprintf("%s must be less than %s", i.getDescription(), i2.getDescription())
	notDescription := fmt.Sprintf("%s must not be less than %s", i.getDescription(), i2.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("f-lt(%s,%s)", i.getDescription(), i2.getDescription()),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := i.getValue(i.v, msg)
			val2 := i2.getValue(i2.v, msg)
			return val1 >= val2, nil
		},
		arguments: []argI{i, i2},
	})
}

func (i *float) LTE(i2 *float) *Rule {
	description := fmt.Sprintf("%s must be less than or equal to %s", i.getDescription(), i2.getDescription())
	notDescription := fmt.Sprintf("%s must not be less than or equal to %s", i.getDescription(), i2.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("f-lte(%s,%s)", i.getDescription(), i2.getDescription()),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := i.getValue(i.v, msg)
			val2 := i2.getValue(i2.v, msg)
			return val1 > val2, nil
		},
		arguments: []argI{i, i2},
	})
}

func (i *float) InRange(min *float, max *float) *Rule {
	description := fmt.Sprintf("%s must be in range %s to %s", i.getDescription(), min.getDescription(), max.getDescription())
	notDescription := fmt.Sprintf("%s must not be in range %s to %s", i.getDescription(), min.getDescription(), max.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("f-ir(%s,%s,%s)", i.getDescription(), min.getDescription(), max.getDescription()),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val := i.getValue(i.v, msg)
			minVal := min.getValue(min.v, msg)
			maxVal := max.getValue(max.v, msg)
			return val < minVal || val > maxVal, nil
		},
		arguments: []argI{i, min, max},
	})
}

func (i *float) Plus(i2 *float) *float {
	description := fmt.Sprintf("(%s + %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return i.getValue(v, msg) + i2.getValue(v, msg)
	}
	return newF
}

func (i *float) Minus(i2 *float) *float {
	description := fmt.Sprintf("(%s - %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return i.getValue(v, msg) - i2.getValue(v, msg)
	}
	return newF
}

func (i *float) Times(i2 *float) *float {
	description := fmt.Sprintf("(%s * %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return i.getValue(v, msg) * i2.getValue(v, msg)
	}
	return newF
}

func (i *float) DividedBy(i2 *float) *float {
	description := fmt.Sprintf("(%s / %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return i.getValue(v, msg) / i2.getValue(v, msg)
	}
	return newF
}

func (i *float) Mod(i2 *float) *float {
	description := fmt.Sprintf("(%s %% %s)", i.getDescription(), i2.getDescription())
	newPaths := append(i.paths, i2.paths...)
	newF := &float{description: description, paths: newPaths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return float64(int64(i.getValue(v, msg)) % int64(i2.getValue(v, msg)))
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
