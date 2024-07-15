package validator

import (
	"fmt"
	"regexp"

	"google.golang.org/protobuf/proto"
)

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
