package validator

import (
	"context"
	"fmt"
	"strings"

	"go.alis.build/alog"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Condition struct {
	Description         string
	shouldRunRule       func(data interface{}, conditionFieldInfos map[string]*FieldInfo) (bool, error)
	shouldValidateField func(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error)
	conditionFields     []string // if empty, the fields from the rule that the condition is applied on is used
	allowedKinds        []protoreflect.Kind
}

func (c *Condition) ShouldRunRule(data interface{}, fieldInfos map[string]*FieldInfo) (bool, error) {
	if c.shouldRunRule == nil {
		return c.shouldRunRule(data, fieldInfos)
	} else {
		return true, nil
	}
}

func (c *Condition) ShouldValidateField(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error) {
	if c.shouldValidateField == nil {
		return c.shouldValidateField(data, fieldPath, fieldInfo)
	} else {
		return true, nil
	}
}

func OR(conditions ...Condition) *Condition {
	descriptions := make([]string, len(conditions))
	for i, cond := range conditions {
		descriptions[i] = cond.Description
	}
	descr := "(" + strings.Join(descriptions, " OR ") + ")"
	return &Condition{
		Description: descr,
		shouldRunRule: func(data interface{}, fieldInfos map[string]*FieldInfo) (bool, error) {
			for _, cond := range conditions {
				run, err := cond.ShouldRunRule(data, fieldInfos)
				if err != nil {
					return false, err
				}
				if run {
					return true, nil
				}
			}
			return false, nil
		}, shouldValidateField: func(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error) {
			for _, cond := range conditions {
				run, err := cond.ShouldValidateField(data, fieldPath, fieldInfo)
				if err != nil {
					return false, err
				}
				if run {
					return true, nil
				}
			}
			return false, nil
		},
	}
}

func AND(conditions ...Condition) *Condition {
	descriptions := make([]string, len(conditions))
	for i, cond := range conditions {
		descriptions[i] = cond.Description
	}
	descr := "(" + strings.Join(descriptions, " AND ") + ")"
	return &Condition{
		Description: descr,
		shouldRunRule: func(data interface{}, fieldInfos map[string]*FieldInfo) (bool, error) {
			for _, cond := range conditions {
				run, err := cond.ShouldRunRule(data, fieldInfos)
				if err != nil {
					return false, err
				}
				if !run {
					return false, nil
				}
			}
			return true, nil
		}, shouldValidateField: func(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error) {
			for _, cond := range conditions {
				run, err := cond.ShouldValidateField(data, fieldPath, fieldInfo)
				if err != nil {
					return false, err
				}
				if !run {
					return false, nil
				}
			}
			return true, nil
		},
	}
}

// NewNotCondition creates a condition that the rule should only be run if some other condition is false
func NOT(condition Condition) *Condition {
	return &Condition{
		Description: "NOT (" + condition.Description + ")",
		shouldRunRule: func(data interface{}, fieldInfos map[string]*FieldInfo) (bool, error) {
			run, err := condition.ShouldRunRule(data, fieldInfos)
			if err != nil {
				return false, err
			}
			return !run, nil
		},
		shouldValidateField: func(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error) {
			run, err := condition.ShouldValidateField(data, fieldPath, fieldInfo)
			if err != nil {
				return false, err
			}
			return !run, nil
		},
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
