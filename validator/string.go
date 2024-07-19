package validator

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type str struct {
	description string
	getValue    func(v *Validator, msg protoreflect.ProtoMessage) string
	paths       []string
	v           *Validator
}

// String value
func StringValue(val string) *str {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) string { return val }
	return &str{description: fmt.Sprintf("'%s'", val), getValue: getValueFunc}
}

// Non nested string field in the message
func StringField(path string) *str {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) string {
		return v.getString(msg, path)
	}
	return &str{description: path, paths: []string{path}, getValue: getValueFunc}
}

func (s *str) String(msg protoreflect.ProtoMessage) string {
	return s.getValue(s.v, msg)
}

func (s *str) fieldPaths() []string {
	return s.paths
}

func (s *str) setValidator(v *Validator) {
	s.v = v
}

func (s *str) getDescription() string {
	return s.description
}

func (s *str) Equals(str *str) *Rule {
	description := fmt.Sprintf("%s must be equal to %s", s.getDescription(), str.getDescription())
	notDescription := fmt.Sprintf("%s must not be equal to %s", s.getDescription(), str.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("s-eq(%s,%s)", s.description, str.description),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := s.String(msg)
			val2 := str.String(msg)
			return val1 != val2, nil
		},
		arguments: []argI{s, str},
	})
}

func (s *str) StartsWith(str *str) *Rule {
	description := fmt.Sprintf("%s must start with %s", s.getDescription(), str.getDescription())
	notDescription := fmt.Sprintf("%s must not start with %s", s.getDescription(), str.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("s-sw(%s,%s)", s.description, str.description),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := s.String(msg)
			val2 := str.String(msg)
			return !strings.HasPrefix(val1, val2), nil
		},
		arguments: []argI{s, str},
	})
}

func (s *str) EndsWith(str *str) *Rule {
	description := fmt.Sprintf("%s must end with %s", s.getDescription(), str.getDescription())
	notDescription := fmt.Sprintf("%s must not end with %s", s.getDescription(), str.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("s-ew(%s,%s)", s.description, str.description),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := s.String(msg)
			val2 := str.String(msg)
			return !strings.HasSuffix(val1, val2), nil
		},
		arguments: []argI{s, str},
	})
}

func (s *str) Contains(str *str) *Rule {
	description := fmt.Sprintf("%s must contain %s", s.getDescription(), str.getDescription())
	notDescription := fmt.Sprintf("%s must not contain %s", s.getDescription(), str.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("s-c(%s,%s)", s.description, str.description),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := s.String(msg)
			val2 := str.String(msg)
			return !strings.Contains(val1, val2), nil
		},
		arguments: []argI{s, str},
	})
}

func (s *str) MatchesRegex(str *str) *Rule {
	description := fmt.Sprintf("%s must match regex %s", s.getDescription(), str.getDescription())
	notDescription := fmt.Sprintf("%s must not match regex %s", s.getDescription(), str.getDescription())
	return NewRule(&Rule{
		Id:             fmt.Sprintf("s-mr(%s,%s)", s.description, str.description),
		Description:    description,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			val1 := s.String(msg)
			val2 := str.String(msg)
			return !strings.Contains(val1, val2), nil
		},
		arguments: []argI{s, str},
	})
}

func (s *str) Length() *integer {
	description := fmt.Sprintf("length of %s", s.getDescription())
	newI := &integer{description: description, paths: s.paths}
	newI.getValue = func(v *Validator, msg protoreflect.ProtoMessage) int64 {
		return int64(len(s.getValue(v, msg)))
	}
	return newI
}

func (s *str) LengthAsFloat() *float {
	description := fmt.Sprintf("length of %s", s.getDescription())
	newF := &float{description: description, paths: s.paths}
	newF.getValue = func(v *Validator, msg protoreflect.ProtoMessage) float64 {
		return float64(len(s.getValue(v, msg)))
	}
	return newF
}
