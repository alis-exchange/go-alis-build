package validator

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.alis.build/alog"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

// This rule will check if the value matches the regex pattern.
func (v *Validator) AddRegexRule(fieldPath string, regex string, skipIfEmpty bool) {
	// validate
	_, err := v.GetStringField(v.protoMsg, fieldPath)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}
	validatorFunc := func(data interface{}, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error) {
		if _, ok := alreadyViolatedFields[fieldPath]; ok {
			return []*pbOpen.Violation{}, nil
		}
		value, err := v.GetStringField(data, fieldPath)
		if err != nil {
			return nil, err
		}
		if value == "" && skipIfEmpty {
			return []*pbOpen.Violation{}, nil
		}
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
	v.AddRule(fmt.Sprintf("%s must match the regex pattern: %s", fieldPath, regex), []string{fieldPath}, validatorFunc)
}

func (v *Validator) AddEmailRule(fieldPath string, skipIfEmpty bool) {
	// validate
	_, err := v.GetStringField(v.protoMsg, fieldPath)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}
	validatorFunc := func(data interface{}, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error) {
		if _, ok := alreadyViolatedFields[fieldPath]; ok {
			return []*pbOpen.Violation{}, nil
		}
		value, err := v.GetStringField(data, fieldPath)
		if err != nil {
			return nil, err
		}
		if value == "" && skipIfEmpty {
			return []*pbOpen.Violation{}, nil
		}
		if !regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(value) {
			return []*pbOpen.Violation{
				{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s is not a valid email", fieldPath),
				},
			}, nil
		}
		return nil, nil
	}
	v.AddRule(fmt.Sprintf("%s must be a valid email", fieldPath), []string{fieldPath}, validatorFunc)
}

func (v *Validator) AddDomainRule(fieldPath string, skipIfEmpty bool) {
	// validate
	_, err := v.GetStringField(v.protoMsg, fieldPath)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}
	validatorFunc := func(data interface{}, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error) {
		if _, ok := alreadyViolatedFields[fieldPath]; ok {
			return []*pbOpen.Violation{}, nil
		}
		value, err := v.GetStringField(data, fieldPath)
		if err != nil {
			return nil, err
		}
		if value == "" && skipIfEmpty {
			return []*pbOpen.Violation{}, nil
		}
		if !regexp.MustCompile(`^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(value) {
			return []*pbOpen.Violation{
				{
					FieldPath: fieldPath,
					Message:   fmt.Sprintf("%s is not a valid domain", fieldPath),
				},
			}, nil
		}
		return nil, nil
	}
	v.AddRule(fmt.Sprintf("%s must be a valid domain", fieldPath), []string{fieldPath}, validatorFunc)
}

func (v *Validator) AddStringAllowedValuesRule(fieldPath string, allowedValues []string, skipIfEmpty bool) {
	// validate
	_, err := v.GetStringField(v.protoMsg, fieldPath)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}
	validatorFunc := func(data interface{}, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error) {
		if _, ok := alreadyViolatedFields[fieldPath]; ok {
			return []*pbOpen.Violation{}, nil
		}
		value, err := v.GetStringField(data, fieldPath)
		if err != nil {
			return nil, err
		}
		if value == "" && skipIfEmpty {
			return []*pbOpen.Violation{}, nil
		}
		for _, allowedValue := range allowedValues {
			if value == allowedValue {
				return []*pbOpen.Violation{}, nil
			}
		}
		return []*pbOpen.Violation{
			{
				FieldPath: fieldPath,
				Message:   fmt.Sprintf("%s must be one of %v", fieldPath, allowedValues),
			},
		}, nil
	}
	v.AddRule(fmt.Sprintf("%s must be one of %v", fieldPath, allowedValues), []string{fieldPath}, validatorFunc)
}

func (v *Validator) AddAipResourceNameRule(fieldPath string, collectionIdentifiers []string, skipIfEmpty bool) {
	// validate
	_, err := v.GetStringField(v.protoMsg, fieldPath)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}
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
	validatorFunc := func(data interface{}, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error) {
		if _, ok := alreadyViolatedFields[fieldPath]; ok {
			return []*pbOpen.Violation{}, nil
		}
		value, err := v.GetStringField(data, fieldPath)
		if err != nil {
			return nil, err
		}
		if value == "" && skipIfEmpty {
			return []*pbOpen.Violation{}, nil
		}
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
	v.AddRule(fmt.Sprintf("%s must match pattern %s with each '*' matching ^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$", fieldPath, pattern), []string{fieldPath}, validatorFunc)
}

func (v *Validator) AddStringLengthRule(fieldPath string, min int, max int, skipIfEmpty bool) {
	validatorFunc := func(data interface{}, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error) {
		if _, ok := alreadyViolatedFields[fieldPath]; ok {
			return []*pbOpen.Violation{}, nil
		}
		value, err := v.GetStringField(data, fieldPath)
		if err != nil {
			return nil, err
		}
		if value == "" && skipIfEmpty {
			return []*pbOpen.Violation{}, nil
		}
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
	v.AddRule(fmt.Sprintf("%s must be between %d and %d characters", fieldPath, min, max), []string{fieldPath}, validatorFunc)
}
