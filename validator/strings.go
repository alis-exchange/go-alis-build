package validator

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.alis.build/alog"
	"google.golang.org/protobuf/reflect/protoreflect"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

// This rule will check if the value matches the regex pattern.
func (v *Validator) AddRegexRule(fieldPath string, regex string, options *Options) {
	// validate
	_, err := v.GetStringField(v.protoMsg, fieldPath)
	if err != nil {
		alog.Fatal(context.Background(), err.Error())
	}
	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		value := fieldInfos[fieldPath].Value.String()
		if !regexp.MustCompile(regex).MatchString(value) {
			return []*pbOpen.Violation{
				{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s does not match %s", fieldPath, regex),
				},
			}, nil
		}
		return nil, nil
	}
	v.AddRule(fmt.Sprintf("%s must match the regex pattern: %s", fieldPath, regex), []string{fieldPath}, validatorFunc, options)
}

func (v *Validator) AddEmailRule(fieldPath string) {
	v.AddEmailRules([]string{fieldPath}, nil)
}

func (v *Validator) AddEmailRules(fieldPaths []string, options *Options) {
	// validate
	v.validateFieldPaths(fieldPaths, protoreflect.StringKind)

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
	description := ""
	if len(fieldPaths) > 1 {
		description = fmt.Sprintf("%s must be valid emails", strings.Join(fieldPaths, " & "))
	} else {
		description = fmt.Sprintf("%s must be a valid email", fieldPaths[0])
	}
	v.AddRule(description, fieldPaths, validatorFunc, options)
}

func (v *Validator) AddDomainRule(fieldPath string, options *Options) {
	v.AddDomainRules([]string{fieldPath}, options)
}

func (v *Validator) AddDomainRules(fieldPaths []string, options *Options) {
	// validate
	v.validateFieldPaths(fieldPaths, protoreflect.StringKind)

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
	description := ""
	if len(fieldPaths) > 1 {
		description = fmt.Sprintf("%s must be valid domains", strings.Join(fieldPaths, " & "))
	} else {
		description = fmt.Sprintf("%s must be a valid domain", fieldPaths[0])
	}
	v.AddRule(description, fieldPaths, validatorFunc, options)
}

func (v *Validator) AddStringAllowedValuesRule(fieldPath string, allowedValues []string, options *Options) {
	// validate
	_, err := v.GetStringField(v.protoMsg, fieldPath)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}
	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		value := fieldInfos[fieldPath].Value.String()
		isValid := false
		for _, allowedValue := range allowedValues {
			if value == allowedValue {
				isValid = true
				break
			}
		}
		if !isValid {
			return []*pbOpen.Violation{
				{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s must be one of %s", fieldPath, strings.Join(allowedValues, ", ")),
				},
			}, nil
		}
		return nil, nil
	}
	v.AddRule(fmt.Sprintf("%s must be one of %s", fieldPath, strings.Join(allowedValues, ", ")), []string{fieldPath}, validatorFunc, options)
}

func (v *Validator) AddAipResourceNameRule(fieldPath string, collectionIdentifiers []string, options *Options) {
	// validate
	v.validateFieldPaths([]string{fieldPath}, protoreflect.StringKind)
	// Collection identifiers must begin with a lower-cased letter and contain only ASCII letters and numbers (/[a-z][a-zA-Z0-9]*/).
	for _, collectionIdentifier := range collectionIdentifiers {
		if !regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`).MatchString(collectionIdentifier) {
			alog.Fatalf(context.Background(), "collection identifier (%s) does not match regex (/[a-z][a-zA-Z0-9]*/)", collectionIdentifier)
		}
	}
	pattern := ""
	for _, collectionIdentifier := range collectionIdentifiers {
		pattern += fmt.Sprintf("%s/*", collectionIdentifier)
	}
	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		value := fieldInfos[fieldPath].Value.String()
		// split string on "/" and ensure all even indexes are collection identifiers and all odd indexes match ^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$
		parts := strings.Split(value, "/")
		collectionIdentifierIndex := 0
		maxLength := len(collectionIdentifiers)*2 - 1
		isValid := true
		if len(parts) > maxLength {
			isValid = false
		}
		for i, part := range parts {
			if i%2 == 0 {
				expectedCollectionIdentifier := collectionIdentifiers[collectionIdentifierIndex]
				if part != expectedCollectionIdentifier {
					isValid = false
				}
			} else {
				if !regexp.MustCompile(`^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`).MatchString(part) {
					isValid = false
				}
			}
		}
		if !isValid {
			return []*pbOpen.Violation{
				{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s must match pattern %s", fieldPath, pattern),
				},
			}, nil
		}
		return []*pbOpen.Violation{}, nil
	}
	v.AddRule(fmt.Sprintf("%s must match pattern %s with each '*' matching ^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$", fieldPath, pattern), []string{fieldPath}, validatorFunc, options)
}

func (v *Validator) AddStringLengthRule(fieldPath string, min int, max int, options *Options) {
	// validate
	v.validateFieldPaths([]string{fieldPath}, protoreflect.StringKind)
	validatorFunc := func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
		value := fieldInfos[fieldPath].Value.String()
		if len(value) < min || len(value) > max {
			return []*pbOpen.Violation{
				{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s must be between %d and %d characters", fieldPath, min, max),
				},
			}, nil
		}
		return nil, nil
	}
	v.AddRule(fmt.Sprintf("%s must be between %d and %d characters", fieldPath, min, max), []string{fieldPath}, validatorFunc, options)
}
