package validator

import (
	"context"
	"fmt"

	"go.alis.build/alog"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

func (v *Validator) AddIntRangeRule(fieldPath string, min int, max int, skipIfEmpty bool) {
	// validate
	_, err := v.GetIntField(v.protoMsg, fieldPath)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}
	validatorFunc := func(data interface{}, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error) {
		if _, ok := alreadyViolatedFields[fieldPath]; ok {
			return []*pbOpen.Violation{}, nil
		}
		value, err := v.GetIntField(data, fieldPath)
		if err != nil {
			return nil, err
		}
		if value == 0 && skipIfEmpty {
			return []*pbOpen.Violation{}, nil
		}
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
	v.AddRule(fmt.Sprintf("%s must be between %d and %d", fieldPath, min, max), []string{fieldPath}, validatorFunc)
}

func (v *Validator) AddFloatRangeRule(fieldPath string, min float64, max float64, skipIfEmpty bool) {
	// validate
	_, err := v.GetFloatField(v.protoMsg, fieldPath)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}
	validatorFunc := func(data interface{}, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error) {
		if _, ok := alreadyViolatedFields[fieldPath]; ok {
			return []*pbOpen.Violation{}, nil
		}
		value, err := v.GetFloatField(data, fieldPath)
		if err != nil {
			return nil, err
		}
		if value == 0 && skipIfEmpty {
			return []*pbOpen.Violation{}, nil
		}
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
	v.AddRule(fmt.Sprintf("%s must be between %f and %f", fieldPath, min, max), []string{fieldPath}, validatorFunc)
}
