package validator

import (
	"context"
	"fmt"
	"strings"

	"go.alis.build/alog"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

// This should always be the first rule in a validator and evaluates if the required fields are populated.
func (v *Validator) AddRequiredFieldsRule(fieldPaths []string) {
	// validate
	for _, fieldPath := range fieldPaths {
		_, err := v.GetFieldValue(v.protoMsg, fieldPath, nil)
		if err != nil {
			alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
		}

	}
	validatorFunc := func(data interface{}, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error) {
		var violations []*pbOpen.Violation
		for _, fieldPath := range fieldPaths {
			if _, ok := alreadyViolatedFields[fieldPath]; ok {
				continue
			}
			fd, err := v.GetFieldDescriptor(data, fieldPath)
			if err != nil {
				violations = append(violations, &pbOpen.Violation{
					FieldPath: fieldPath,
					Message:   "required field is missing",
				})
			}

			if !fd.HasPresence() {
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
	v.AddRule(fmt.Sprintf("%s are required fields", strings.Join(fieldPaths, ",")), fieldPaths, validatorFunc)
}
