package validator

import (
	"fmt"

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
