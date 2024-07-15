package validator

import (
	"fmt"
	"reflect"
	"strings"

	"google.golang.org/protobuf/proto"
)

// Helper function to get the field value using reflection
func getField(data proto.Message, fieldPath string) (string, error) {
	// Split the field path into individual field names
	fields := strings.Split(fieldPath, ".")

	// Use reflection to traverse the proto message structure
	var currentField reflect.Value = reflect.ValueOf(data)
	for _, field := range fields {
		// Get the field by name
		currentField = currentField.FieldByName(field)
		if !currentField.IsValid() {
			return "", fmt.Errorf("field %s not found", fieldPath)
		}
	}

	// Get the field value as a string
	fieldValue := currentField.Interface().(string) // Assuming the field is a string
	return fieldValue, nil
}
