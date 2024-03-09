package validator

import (
	"fmt"
	"regexp"

	"google.golang.org/protobuf/proto"
)

type requiredFields struct {
	fieldPaths []string
}

func (c requiredFields) Do(data proto.Message) []Violation {
	// For each of the provided fields, ensure they are populated
	violations := []Violation{}
	for fieldPath := range c.fieldPaths {
		// If the field does not exist, generate a violation
		// TODO: use reflection to get the details...
		violations = append(violations, Violation{
			Message: fmt.Sprintf("field %d is required", fieldPath),
		})
	}
	return violations
}

// RequiredFields adds required field validations to your specified data.
func RequiredFields(fieldPaths []string) requiredFields {
	return requiredFields{fieldPaths: fieldPaths}
}

type bufProtoValidate struct{}

func (c bufProtoValidate) Do(data proto.Message) []Violation {
	// We'll use the github.com/bufbuild/protovalidate-go pacakge to validate a message
	// For each of the provided fields, ensure they are populated
	violations := []Violation{
		{
			Message: "Testing buf protovalidate",
		},
	}
	return violations
}

// BufProtoValidate uses the Buf proto options to create a set of validations.
// This method makes use of the we'll use the github.com/bufbuild/protovalidate-go pacakge
// to validate a message
func BufProtoValidate() bufProtoValidate {
	return bufProtoValidate{}
}

type regexField struct {
	fieldPaths  []string
	expressions []string
}

func (c regexField) Do(data proto.Message) []Violation {
	violations := []Violation{}
	for i, fieldPath := range c.fieldPaths {
		// validate the Name field using regex
		// User refection to get the value of the field.
		fieldValue := "DUMMY VALUE"

		validatedName := regexp.MustCompile(c.expressions[i]).MatchString(fieldValue)
		if !validatedName {
			violations = append(violations, Violation{
				FieldPath: fieldPath,
				Message:   fmt.Sprintf("value (%s) is not of the right format: %s", fieldValue, c.expressions[i]),
			})
		}
	}

	return violations
}

// RegexField adds required field validations to your specified data.
func RegexFields(fieldPaths []string, expressions []string) regexField {
	return regexField{
		fieldPaths:  fieldPaths,
		expressions: expressions,
	}
}
