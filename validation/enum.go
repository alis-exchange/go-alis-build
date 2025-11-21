package validation

import "google.golang.org/protobuf/reflect/protoreflect"

// Enum provides validation rules for enum values.
type Enum struct {
	standard[protoreflect.Enum]
}

// Is adds a rule asserting that the enum value must be equal to the given value.
func (e *Enum) Is(value protoreflect.Enum) *Enum {
	e.add("be %v", "is %v", e.value == value, value)
	return e
}

// IsSpecified adds a rule asserting that the enum value must not be the zero value (unspecified).
func (e *Enum) IsSpecified() *Enum {
	e.add("be specified", "is specified", e.value.Number() != 0)
	return e
}

// IsUnspecified adds a rule asserting that the enum value must be the zero value (unspecified).
func (e *Enum) IsUnspecified() *Enum {
	e.add("not be specified", "is not specified", e.value.Number() == 0)
	return e
}

// IsOneof adds a rule asserting that the enum value must be one of the given values.
func (e *Enum) IsOneof(values ...protoreflect.Enum) *Enum {
	satisfied := false
	for _, v := range values {
		if e.value == v {
			satisfied = true
			break
		}
	}
	e.add("be one of %v", "is one of %v", satisfied, values)
	return e
}

// IsNoneof adds a rule asserting that the enum value must not be any of the given values.
func (e *Enum) IsNoneof(values ...protoreflect.Enum) *Enum {
	satisfied := true
	for _, v := range values {
		if e.value == v {
			satisfied = false
			break
		}
	}
	e.add("be none of %v", "is none of %v", satisfied, values)
	return e
}
