package validator

import (
	"context"
	"fmt"
	"strings"

	"go.alis.build/alog"
	"google.golang.org/protobuf/reflect/protoreflect"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

type Condition struct {
	Description     string
	NotDescription string
	getViolations   func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error)
	conditionFields []string // if empty, the fields from the rule that the condition is applied on is used
	allowedKinds    []protoreflect.Kind
}

func OR(conditions ...Condition) *Condition {
	descriptions := make([]string, len(conditions))
	notDescriptions := make([]string, len(conditions))
	for i, cond := range conditions {
		descriptions[i] = cond.Description
		notDescriptions[i] = cond.NotDescription
	}
	descr := "(" + strings.Join(descriptions, " OR ") + ")"
	notDescription := "(" + strings.Join(notDescriptions, " AND ") + ")"
	return &Condition{
		Description: descr,
		NotDescription: notDescription,
		getViolations: func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
			allViols := []*pbOpen.Violation{}
			for _, cond := range conditions {
				violations, err := cond.getViolations(data, fieldInfos)
				if err != nil {
					return violations, err
				}
				if len(violations) == 0 {
					return violations, nil
				}
				allViols = append(allViols, violations...)
			}
			return allViols, nil
		},
	}
}

func AND(conditions ...Condition) *Condition {
	descriptions := make([]string, len(conditions))
	notDescriptions := make([]string, len(conditions))
	for i, cond := range conditions {
		descriptions[i] = cond.Description
		notDescriptions[i] = cond.NotDescription
	}
	descr := "(" + strings.Join(descriptions, " AND ") + ")"
	notDescription := "(" + strings.Join(notDescriptions, " OR ") + ")"
	return &Condition{
		Description: descr,
		NotDescription: notDescription,
		getViolations: func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
			allViols := []*pbOpen.Violation{}
			for _, cond := range conditions {
				violations, err := cond.getViolations(data, fieldInfos)
				if err != nil {
					return violations, err
				}
				if len(violations) > 0 {
					return violations, nil
				}
				allViols = append(allViols, violations...)
			}
			return allViols, nil
		},
	}
}

// NewNotCondition creates a condition that the rule should only be run if some other condition is false
func NOT(condition Condition) *Condition {
	return &Condition{
		Description: condition.NotDescription,
		NotDescription: condition.Description,
		getViolations: func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
			violations, err := condition.getViolations(data, fieldInfos)
			if err != nil {
				return violations, err
			}
			if len(violations) == 0 {
				for fieldPath,_ := range fieldInfos {
					violations = append(violations, &pbOpen.Violation{
						FieldPath: fieldPath,
						Message: fmt.Sprintf(),
					}
				}
			}
			return nil, nil
		
		}
	}
}

// ------------------------------- //

func EnumIs(fieldPath string, val interface{}) *Condition {
	// enumValue := val.(protoreflect.Enum)
	enumValue, ok := val.(protoreflect.Enum)
	if !ok {
		alog.Fatalf(context.Background(), "val is not a protoreflect.Enum")
	}
	value := enumValue.Number()
	description := fmt.Sprintf("%s is equal to %s", fieldPath, enumValue.Descriptor().Values().Get(int(value)).Name())
	return &Condition{
		Description: description,
		shouldRunRule: func(data interface{}, conditionFieldInfos map[string]*FieldInfo) (bool, error) {
			return conditionFieldInfos[fieldPath].Value.Enum() == value, nil
		},
		allowedKinds:    []protoreflect.Kind{protoreflect.EnumKind},
		conditionFields: []string{fieldPath},
	}
}

func (v *Validator) IsPopulated(fieldPath string) *Condition {
	description := fmt.Sprintf("%s is populated", fieldPath)
	return &Condition{
		Description: description,
		shouldRunRule: func(data interface{}, conditionFieldInfos map[string]*FieldInfo) (bool, error) {
			fieldInfo := conditionFieldInfos[fieldPath]
			empty := fieldInfo.Value.Equal(fieldInfo.Descriptor.Default())
			return !empty, nil
		},
	}
}
