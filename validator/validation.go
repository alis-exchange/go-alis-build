package validator

import (
	"go.alis.build/authz"
	"google.golang.org/protobuf/reflect/protoreflect"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

type (
	Validation struct {
		rule               *pbOpen.Rule
		conditions         []*Condition
		function           Func
		authorizedFunction AuthorizedFunc
		repeatedPaths      []string
	}

	Func           func(data interface{}, fields map[string]*FieldInfo) ([]*pbOpen.Violation, error)
	AuthorizedFunc func(data interface{}, fields map[string]*FieldInfo, authInfo *authz.AuthInfo) ([]*pbOpen.Violation, error)

	FieldInfo struct {
		Value      protoreflect.Value
		Descriptor protoreflect.FieldDescriptor
		// A field may be skipped if it is already violated or its empty or some condition says to not run
		// Function may choose to ignore this field but violation builder will ignore violations for this field
		Skip bool
	}
)

func (v *Validation) shouldRunRule(data interface{}, fieldInfoCache map[string]*FieldInfo) (bool, error) {
	for _, cond := range v.conditions {

		run, err := cond.ShouldRunRule(data)
		if err != nil {
			return false, err
		}
		if !run {
			return false, nil
		}
	}
	return true, nil
}

func (v *Validation) shouldValidateField(data interface{}, fieldPath string, fieldInfo *FieldInfo) (bool, error) {
	for _, cond := range v.conditions {
		run, err := cond.ShouldValidateField(data, fieldPath, fieldInfo)
		if err != nil {
			return false, err
		}
		if !run {
			return false, nil
		}
	}
	return true, nil
}

func (v *Validation) run(data interface{}, fields map[string]*FieldInfo, authInfo *authz.AuthInfo) ([]*pbOpen.Violation, error) {
	if authInfo != nil {
		return v.authorizedFunction(data, fields, authInfo)
	} else {
		return v.function(data, fields)
	}
}

func (v *Validation) ApplyIf(cond *Condition) {
	v.conditions = append(v.conditions, cond)
}

func (v *Validation) IgnoreIf(cond *Condition) {
	v.conditions = append(v.conditions, NOT(cond))
}
