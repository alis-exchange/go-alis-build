package validator

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Rule is only applied if following conditions are met:
// - cond1.Description
// - cond2.Description
// ...

type Condition interface {
	GetDescription() string
	ShouldRunRule(data interface{}) (bool, error)
	ShouldValidateField(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error)
}

// ------------------------------- //
type BasicCondition struct {
	SkipEmptyFields               bool
	ValidateAlreadyViolatedFields bool
}

func (bc *BasicCondition) GetDescription() string {
	return "evaluated field is populated"
}

func (bc *BasicCondition) ShouldRunRule(data interface{}) (bool, error) {
	return true, nil
}

func (bc *BasicCondition) ShouldValidateField(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error) {
	if bc.SkipEmptyFields && fieldInfo.Value.Equal(fieldInfo.Descriptor.Default()) {
		return false, nil
	}
	return true, nil
}

// ------------------------------- //
type RuleCondition struct {
	Description   string
	shouldRunRule func(data interface{}) (bool, error)
}

func (rc *RuleCondition) GetDescription() string {
	return rc.Description
}

func (rc *RuleCondition) ShouldRunRule(data interface{}) (bool, error) {
	return rc.shouldRunRule(data)
}

func (rc *RuleCondition) ShouldValidateField(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error) {
	return true, nil
}

// ------------------------------- //

type FieldCondition struct {
	Description         string
	shouldValidateField func(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error)
}

func (fc *FieldCondition) GetDescription() string {
	return fc.Description
}

func (fc *FieldCondition) ShouldRunRule(data interface{}) (bool, error) {
	return true, nil
}

func (fc *FieldCondition) ShouldValidateField(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error) {
	return fc.shouldValidateField(data, fieldPath, fieldInfo)
}

// ------------------------------- //
type OrCondition struct {
	Description string
	conditions  []Condition
}

func NewOrCondition(conditions ...Condition) *OrCondition {
	descriptions := make([]string, len(conditions))
	for i, cond := range conditions {
		descriptions[i] = cond.GetDescription()
	}
	descr := strings.Join(descriptions, " OR ")
	return &OrCondition{conditions: conditions, Description: descr}
}

func (oc *OrCondition) GetDescription() string {
	return oc.Description
}

func (ac *OrCondition) ShouldRunRule(data interface{}) (bool, error) {
	for _, cond := range ac.conditions {
		run, err := cond.ShouldRunRule(data)
		if err != nil {
			return false, err
		}
		if run {
			return true, nil
		}
	}
	return false, nil
}

func (ac *OrCondition) ShouldEvaluateField(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error) {
	for _, cond := range ac.conditions {
		run, err := cond.ShouldValidateField(data, fieldPath, fieldInfo)
		if err != nil {
			return false, err
		}
		if run {
			return true, nil
		}
	}
	return false, nil
}

// ------------------------------- //
type NotCondition struct {
	Description string
	conditions  Condition
}

// NewNotCondition creates a condition that the rule should only be run if some other condition is false
func NewNotCondition(condition Condition) *NotCondition {
	return &NotCondition{conditions: condition, Description: fmt.Sprintf("NOT(%s)", condition.GetDescription())}
}

func (nc *NotCondition) GetDescription() string {
	return nc.Description
}

func (nc *NotCondition) ShouldRunRule(data interface{}) (bool, error) {
	run, err := nc.conditions.ShouldRunRule(data)
	if err != nil {
		return false, err
	}
	return !run, nil
}

func (nc *NotCondition) ShouldValidateField(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error) {
	run, err := nc.conditions.ShouldValidateField(data, fieldPath, fieldInfo)
	if err != nil {
		return false, err
	}
	return !run, nil
}

// ------------------------------- //

func (v *Validator) NewEnumCondition(fieldPath string, val interface{}) *RuleCondition {
	// enumValue := val.(protoreflect.Enum)
	enumValue, ok := val.(protoreflect.Enum)
	if !ok {
		panic("val is not a protoreflect.Enum")
	}
	v.validateFieldPath(fieldPath, protoreflect.EnumKind)
	value := enumValue.Number()
	description := fmt.Sprintf("%s is equal to %s", fieldPath, enumValue.Descriptor().Values().Get(int(value)).Name())
	return &RuleCondition{
		Description: description,
		shouldRunRule: func(data interface{}) (bool, error) {
			fieldInfo, err := v.GetFieldInfo(data, fieldPath, nil)
			if err != nil {
				return false, err
			}
			return fieldInfo.Value.Enum() == value, nil
		},
	}
}

func (v *Validator) NewIsPopulatedCondition(fieldPath string) *RuleCondition {
	v.validateFieldPath(fieldPath)
	description := fmt.Sprintf("%s is populated", fieldPath)
	return &RuleCondition{
		Description: description,
		shouldRunRule: func(data interface{}) (bool, error) {
			fieldInfo, err := v.GetFieldInfo(data, fieldPath, nil)
			if err != nil {
				return false, err
			}
			empty := fieldInfo.Value.Equal(fieldInfo.Descriptor.Default())
			return !empty, nil
		},
	}
}
