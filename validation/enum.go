package validation

import "google.golang.org/protobuf/reflect/protoreflect"

// Provides rules applicable to enum values.
type Enum struct {
	standard[protoreflect.Enum]
}

// Adds a rule to the parent validator asserting that the enum value is populated.
func (e *Enum) Is(value protoreflect.Enum) *Enum {
	e.add("be %v", "is %v", e.value == value, value)
	return e
}

// Adds a rule to the parent validator asserting that the enum value is populated.
func (e *Enum) IsPopulated() *Enum {
	e.add("be populated", "is populated", e.value.Number() != 0)
	return e
}

// Adds a rule to the parent validator asserting that the enum value is not populated.
func (e *Enum) IsNotPopulated() *Enum {
	e.add("not be populated", "is not populated", e.value.Number() == 0)
	return e
}

// Adds a rule to the parent validator asserting that the enum value is one of the given values.
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

// Adds a rule to the parent validator asserting that the enum value is not one of the given values.
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
