package validator

import (
	"context"
	"fmt"

	"go.alis.build/alog"
	"go.alis.build/authz"
	"google.golang.org/protobuf/reflect/protoreflect"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

func (v *Validator) addRule(idPrefix string, ruleDescription string, fieldPaths []string, validationFunction Func, authorizedValidationFunction AuthorizedFunc, repeatedPaths []string, conditions ...Condition) *Validation {
	if idPrefix == "" {
		alog.Fatalf(context.Background(), "idPrefix is empty")
	}
	if ruleDescription == "" {
		alog.Fatalf(context.Background(), "ruleDescription is empty")
	}
	if len(fieldPaths) == 0 {
		alog.Fatalf(context.Background(), "fieldPaths is empty")
	}
	if validationFunction == nil && authorizedValidationFunction == nil {
		alog.Fatalf(context.Background(), "validationFunction is nil")
	}
	rule := &pbOpen.Rule{
		Id:          fmt.Sprintf("%s-%d", idPrefix, len(v.validations)+1),
		Description: ruleDescription,
		FieldPaths:  fieldPaths,
	}
	validation := &Validation{
		rule:               rule,
		conditions:         conditions,
		function:           validationFunction,
		authorizedFunction: authorizedValidationFunction,
		repeatedPaths:      repeatedPaths,
	}
	if authorizedValidationFunction != nil {
		v.authorizedValidations = append(v.authorizedValidations, validation)
	} else {
		v.validations = append(v.validations, validation)
	}
	return validation
}

func (v *Validator) runValidations(msg interface{}, alreadyViolatedFields map[string]bool, fieldInfoCache map[string]*FieldInfo, authInfo *authz.AuthInfo) ([]*pbOpen.Violation, error) {
	validations := v.validations
	if authInfo != nil {
		validations = v.authorizedValidations
	}
	allViolations := []*pbOpen.Violation{}
	for _, val := range validations {
		runRule, err := val.shouldRunRule(msg)
		if err != nil {
			return nil, err
		}
		if !runRule {
			continue
		}
		fieldInfoMap := make(map[string]*FieldInfo)
		hasAtLeastOneNonSkippedField := false
		for _, fieldPath := range val.rule.FieldPaths {
			fieldInfo, err := v.GetFieldInfo(msg, fieldPath, fieldInfoCache)
			if err != nil {
				return nil, err
			}
			if !fieldInfo.Skip {
				runRuleOnField, err := val.shouldValidateField(msg, fieldPath, fieldInfo)
				if err != nil {
					return nil, err
				}
				if runRuleOnField {
					hasAtLeastOneNonSkippedField = true
				}
			}

			if val.repeatedPaths != nil && len(val.repeatedPaths) > 0 {
				// extract list
				list := fieldInfo.Value.List()
				for i := 0; i < list.Len(); i++ {
					listField := list.Get(i)
					listFieldPath := fmt.Sprintf("%s[%d]", fieldPath, i)
					if _, ok := alreadyViolatedFields[listFieldPath]; ok {
						continue
					}
					if _, ok := fieldInfoMap[listFieldPath]; ok {
						continue
					}
					listFieldInfo := FieldInfo{
						Descriptor: fieldInfo.Descriptor,
						Value:      listField,
					}
					fieldInfoMap[listFieldPath] = &listFieldInfo
				}
			} else {
				fieldInfoMap[fieldPath] = fieldInfo
			}
		}
		if !hasAtLeastOneNonSkippedField {
			continue
		}
		violations, err := val.run(msg, fieldInfoMap, authInfo)
		if err != nil {
			return nil, err
		}
		for _, viol := range violations {
			_, ok := alreadyViolatedFields[viol.FieldPath]
			if ok {
				continue
			}
			viol.RuleId = val.rule.Id
			alreadyViolatedFields[viol.FieldPath] = true
			fieldInfoCache[viol.FieldPath].Skip = true
			allViolations = append(allViolations, viol)
		}
	}
	return allViolations, nil
}

func (v *Validator) validateFieldPaths(fieldPaths []string, allowedTypes ...protoreflect.Kind) []string {
	repeatedPaths := []string{}
	for _, fieldPath := range fieldPaths {
		repPaths := v.validateFieldPath(fieldPath, allowedTypes...)
		repeatedPaths = append(repeatedPaths, repPaths...)
	}
	return repeatedPaths
}

func (v *Validator) validateFieldPath(fieldPath string, allowedTypes ...protoreflect.Kind) []string {
	fi, err := v.GetFieldInfo(v.protoMsg, fieldPath, nil)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}

	if len(allowedTypes) > 0 {
		foundType := false
		for _, allowedType := range allowedTypes {
			if fi.Descriptor.Kind() == allowedType {
				foundType = true
				break
			}
		}
		if !foundType {
			alog.Fatalf(context.Background(), "field path (%s) is not a valid %v field for %s", fieldPath, allowedTypes, v.msgType)
		}
	}

	if fi.Descriptor.Cardinality() == protoreflect.Repeated {
		return []string{fieldPath}
	} else {
		return []string{}
	}
}

func (v *Validator) validateListField(fieldPath string, allowedTypes ...protoreflect.Kind) {
	fi, err := v.GetFieldInfo(v.protoMsg, fieldPath, nil)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}
	if fi.Descriptor.Cardinality() != protoreflect.Repeated {
		alog.Fatalf(context.Background(), "field path (%s) is not a list for %s", fieldPath, v.msgType)
	}
	if len(allowedTypes) > 0 {
		foundType := false
		for _, allowedType := range allowedTypes {
			if fi.Descriptor.Kind() == allowedType {
				foundType = true
				break
			}
		}
		if !foundType {
			alog.Fatalf(context.Background(), "field path (%s) is not a valid %v field for %s", fieldPath, allowedTypes, v.msgType)
		}
	}
}

func locateValidator(data interface{}) (*Validator, bool) {
	protoMsg := data.(protoreflect.ProtoMessage)
	msgType := GetMsgType(protoMsg)
	v, ok := validators[msgType]
	if !ok {
		return nil, false
	}
	return v, true
}
