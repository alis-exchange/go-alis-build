package validator

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

func (v *Validator) AddIntRangeRule(fieldPath string, min int64, max int64, options *Options) {
	v.validateFieldPath(fieldPath, protoreflect.Int32Kind, protoreflect.Int64Kind)
	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		value := fieldInfos[fieldPath].Value.Int()
		if value < min || value > max {
			return []*pbOpen.Violation{
				{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s must be between %d and %d", fieldPath, min, max),
				},
			}, nil
		}
		return nil, nil
	}
	v.AddRule(fmt.Sprintf("%s must be between %d and %d", fieldPath, min, max), []string{fieldPath}, validatorFunc, options)
}

func (v *Validator) AddFloatRangeRule(fieldPath string, min float64, max float64, options *Options) {
	v.validateFieldPath(fieldPath, protoreflect.FloatKind, protoreflect.DoubleKind)
	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		value := fieldInfos[fieldPath].Value.Float()
		if value < min || value > max {
			return []*pbOpen.Violation{
				{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s must be between %f and %f", fieldPath, min, max),
				},
			}, nil
		}
		return nil, nil
	}
	v.AddRule(fmt.Sprintf("%s must be between %f and %f", fieldPath, min, max), []string{fieldPath}, validatorFunc, options)
}
