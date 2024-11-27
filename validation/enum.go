package validation

import "google.golang.org/protobuf/reflect/protoreflect"

type Enum struct {
	standard[protoreflect.Enum]
}

func (e *Enum) Is(value protoreflect.Enum) *Enum {
	e.add("be %v", "is %v", e.value == value, value)
	return e
}

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
