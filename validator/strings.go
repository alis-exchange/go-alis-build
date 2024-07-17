package validator

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

// This rule will check if the value matches the regex pattern.
// Supports repeated fields.
func (v *Validator) AddRegexRule(fieldPath string, regex string, conditions ...Condition) {
	repPaths := v.validateFieldPath(fieldPath, protoreflect.StringKind)
	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		for path, fieldInfo := range fieldInfos {
			if !regexp.MustCompile(regex).MatchString(fieldInfo.Value.String()) {
				return []*pbOpen.Violation{
					{
						FieldPath: fieldPath,
						Message:   fmt.Sprintf("%s does not match %s", path, regex),
					},
				}, nil
			}
		}
		return nil, nil
	}
	v.addRule("regex", fmt.Sprintf("%s must match the regex pattern: %s", fieldPath, regex), []string{fieldPath}, validatorFunc, nil, repPaths, conditions...)
}

func (v *Validator) AddEmailRule(fieldPath string) {
	v.AddEmailRules([]string{fieldPath}, nil)
}

func (v *Validator) AddEmailRules(fieldPaths []string, conditions ...Condition) {
	// validate
	repPaths := v.validateFieldPaths(fieldPaths, protoreflect.StringKind)

	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		violations := []*pbOpen.Violation{}
		for fieldPath, fieldInfo := range fieldInfos {
			value := fieldInfo.Value.String()
			if !regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(value) {
				violations = append(violations, &pbOpen.Violation{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s is not a valid email", fieldPath),
				})
			}
		}
		return violations, nil
	}
	description := getDescription("email", fieldPaths, repPaths)
	v.addRule("email", description, fieldPaths, validatorFunc, nil, repPaths, conditions...)
}

func getDescription(subject string, fieldPaths []string, repPaths []string) string {
	if len(fieldPaths) > 1 || len(repPaths) > 1 {
		subject = fmt.Sprintf("%s fields", subject)
		return fmt.Sprintf("%s must contain valid %s", strings.Join(fieldPaths, " & "), subject)
	} else {
		return fmt.Sprintf("%s must be a valid %s", fieldPaths[0], subject)
	}
}

func (v *Validator) AddDomainRule(fieldPath string, conditions ...Condition) {
	v.AddDomainRules([]string{fieldPath}, conditions...)
}

func (v *Validator) AddDomainRules(fieldPaths []string, conditions ...Condition) {
	// validate
	repPaths := v.validateFieldPaths(fieldPaths, protoreflect.StringKind)

	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		violations := []*pbOpen.Violation{}
		for fieldPath, fieldInfo := range fieldInfos {
			value := fieldInfo.Value.String()
			if !regexp.MustCompile(`^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(value) {
				violations = append(violations, &pbOpen.Violation{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s is not a valid domain", fieldPath),
				})
			}
		}
		return violations, nil
	}
	description := getDescription("domain", fieldPaths, repPaths)
	v.addRule("domain", description, fieldPaths, validatorFunc, nil, repPaths, conditions...)
}

// func (v *Validator) AddStringAllowedValuesRule(fieldPath string, allowedValues []string, conditions ...Condition) {
// 	// validate
// 	_, err := v.GetStringField(v.protoMsg, fieldPath)
// 	if err != nil {
// 		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
// 	}
// 	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
// 		value := fieldInfos[fieldPath].Value.String()
// 		isValid := false
// 		for _, allowedValue := range allowedValues {
// 			if value == allowedValue {
// 				isValid = true
// 				break
// 			}
// 		}
// 		if !isValid {
// 			return []*pbOpen.Violation{
// 				{
// 					FieldPath: fieldPath,
// 					Message:   fmt.Sprintf("%s must be one of %s", fieldPath, strings.Join(allowedValues, ", ")),
// 				},
// 			}, nil
// 		}
// 		return nil, nil
// 	}
// 	v.addRule("allowed-string", fmt.Sprintf("%s must be one of %s", fieldPath, strings.Join(allowedValues, ", ")), []string{fieldPath}, validatorFunc, conditions...)
// }

// func (v *Validator) AddAipResourceNameRule(fieldPath string, collectionIdentifiers []string, conditions ...Condition) {
// 	// validate
// 	v.validateFieldPath(fieldPath, protoreflect.StringKind)
// 	// Collection identifiers must begin with a lower-cased letter and contain only ASCII letters and numbers (/[a-z][a-zA-Z0-9]*/).
// 	for _, collectionIdentifier := range collectionIdentifiers {
// 		if !regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`).MatchString(collectionIdentifier) {
// 			alog.Fatalf(context.Background(), "collection identifier (%s) does not match regex (/[a-z][a-zA-Z0-9]*/)", collectionIdentifier)
// 		}
// 	}
// 	pattern := ""
// 	for _, collectionIdentifier := range collectionIdentifiers {
// 		pattern += fmt.Sprintf("%s/*/", collectionIdentifier)
// 	}
// 	// remove last "/"
// 	pattern = pattern[:len(pattern)-1]
// 	errorMsg := fmt.Sprintf("%s must match pattern %s with each '*' matching ^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$", fieldPath, pattern)
// 	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
// 		value := fieldInfos[fieldPath].Value.String()
// 		// split string on "/" and ensure all even indexes are collection identifiers and all odd indexes match ^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$
// 		parts := strings.Split(value, "/")
// 		collectionIdentifierIndex := 0
// 		maxLength := len(collectionIdentifiers) * 2
// 		isValid := true
// 		if len(parts) > maxLength {
// 			isValid = false
// 		}
// 		for i, part := range parts {
// 			if i%2 == 0 {
// 				expectedCollectionIdentifier := collectionIdentifiers[collectionIdentifierIndex]
// 				if part != expectedCollectionIdentifier {
// 					isValid = false
// 				}
// 				collectionIdentifierIndex++
// 			} else {
// 				if !regexp.MustCompile(`^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`).MatchString(part) {
// 					isValid = false
// 				}
// 			}
// 			if !isValid {
// 				break
// 			}

// 		}
// 		if !isValid {
// 			return []*pbOpen.Violation{
// 				{
// 					FieldPath: fieldPath,
// 					Message:   errorMsg,
// 				},
// 			}, nil
// 		}
// 		return []*pbOpen.Violation{}, nil
// 	}
// 	v.addRule("aip-resource-name", errorMsg, []string{fieldPath}, validatorFunc, conditions...)
// }

// func (v *Validator) AddStringLengthRule(fieldPath string, min int, max int, conditions ...Condition) {
// 	// validate
// 	v.validateFieldPaths([]string{fieldPath}, protoreflect.StringKind)
// 	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
// 		value := fieldInfos[fieldPath].Value.String()
// 		if len(value) < min || len(value) > max {
// 			return []*pbOpen.Violation{
// 				{
// 					FieldPath: fieldPath,
// 					Message:   fmt.Sprintf("%s must be between %d and %d characters", fieldPath, min, max),
// 				},
// 			}, nil
// 		}
// 		return nil, nil
// 	}
// 	v.addRule("string-length", fmt.Sprintf("%s must be between %d and %d characters", fieldPath, min, max), []string{fieldPath}, validatorFunc, conditions...)
// }
