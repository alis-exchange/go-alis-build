package validator

import (
	"fmt"
	"strings"

	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

// This should always be the first rule in a validator and evaluates if the required fields are populated.
func (v *Validator) AddRequiredFieldsRule(fieldPaths []string, conditions ...Condition) {
	// validate
	v.validateFieldPaths(fieldPaths)
	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		var violations []*pbOpen.Violation
		for fieldPath, fieldInfo := range fieldInfos {
			if fieldInfo.Descriptor.Default().Equal(fieldInfo.Value) {
				violations = append(violations, &pbOpen.Violation{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s is required", fieldPath),
				})
			}
		}
		if len(violations) > 0 {
			return violations, nil
		}
		return violations, nil
	}
	v.addRule("required-fields", fmt.Sprintf("%s are required fields", strings.Join(fieldPaths, ",")), fieldPaths, validatorFunc, nil, []string{}, conditions...)
}

func (v *Validator) AddListLengthRule(fieldPath string, min int, max int, conditions ...Condition) {
	v.validateListField(fieldPath)
	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		value := fieldInfos[fieldPath].Value.List()
		if value.Len() < min || value.Len() > max {
			return []*pbOpen.Violation{
				{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s must have between %d and %d items", fieldPath, min, max),
				},
			}, nil
		}
		return nil, nil
	}
	v.addRule("list-length", fmt.Sprintf("%s must have between %d and %d items", fieldPath, min, max), []string{fieldPath}, validatorFunc, nil, []string{}, conditions...)
}
