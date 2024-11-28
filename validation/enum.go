package validation

import "google.golang.org/protobuf/reflect/protoreflect"

// Provides rules applicable to enum values.
type Enum struct {
	standard[protoreflect.Enum]
}

// Adds a rule to the parent validator asserting that the enum value is populated.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (e *Enum) Is(value protoreflect.Enum) *Enum {
	e.add("be %v", "is %v", e.value == value, value)
	return e
}

// Adds a rule to the parent validator asserting that the enum value is specified.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (e *Enum) IsSpecified() *Enum {
	e.add("be specified", "is specified", e.value.Number() != 0)
	return e
}

// Adds a rule to the parent validator asserting that the enum value is not specified.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (e *Enum) IsUnspecified() *Enum {
	e.add("not be specified", "is not specified", e.value.Number() == 0)
	return e
}

// Adds a rule to the parent validator asserting that the enum value is one of the given values.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
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
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
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
