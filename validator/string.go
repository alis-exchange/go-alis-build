package validator

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type str struct {
	paths       []string
	getValue    func(v *Validator, msg protoreflect.ProtoMessage) string
	description string
	v           *Validator
}

func (s *str) fieldPaths() []string {
	return s.paths
}

func (s *str) getDescription() string {
	return s.description
}

func (s *str) getValidator() *Validator {
	return s.v
}

func (s *str) setValidator(v *Validator) {
	s.getValue(v, v.protoMsg)
	s.v = v
}

func String(val string) *str {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) string {
		return val
	}
	return &str{description: fmt.Sprintf("'%s'", val), getValue: getValueFunc}
}

func StringField(path string) *str {
	getValueFunc := func(v *Validator, msg protoreflect.ProtoMessage) string {
		return v.getString(msg, path)
	}
	return &str{description: path, getValue: getValueFunc, paths: []string{path}}
}

func (s *str) Equals(str *str) *Rule {
	id := fmt.Sprintf("s-eq(%s,%s)", s.description, str.description)
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be equal to %s", s.getDescription(), str.getDescription()),
		notRule:      fmt.Sprintf("%s must not be equal to %s", s.getDescription(), str.getDescription()),
		condition:    fmt.Sprintf("%s equals %s", s.getDescription(), str.getDescription()),
		notCondition: fmt.Sprintf("%s does not equal %s", s.getDescription(), str.getDescription()),
	}
	args := []argI{s, str}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := s.getValue(s.v, msg)
		val2 := str.getValue(str.v, msg)
		return val1 != val2, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (s *str) StartsWith(str *str) *Rule {
	id := fmt.Sprintf("s-sw(%s,%s)", s.description, str.description)
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must start with %s", s.getDescription(), str.getDescription()),
		notRule:      fmt.Sprintf("%s must not start with %s", s.getDescription(), str.getDescription()),
		condition:    fmt.Sprintf("%s starts with %s", s.getDescription(), str.getDescription()),
		notCondition: fmt.Sprintf("%s does not start with %s", s.getDescription(), str.getDescription()),
	}
	args := []argI{s, str}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := s.getValue(s.v, msg)
		val2 := str.getValue(str.v, msg)
		return !strings.HasPrefix(val1, val2), nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (s *str) EndsWith(str *str) *Rule {
	id := fmt.Sprintf("s-ew(%s,%s)", s.description, str.description)
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must end with %s", s.getDescription(), str.getDescription()),
		notRule:      fmt.Sprintf("%s must not end with %s", s.getDescription(), str.getDescription()),
		condition:    fmt.Sprintf("%s ends with %s", s.getDescription(), str.getDescription()),
		notCondition: fmt.Sprintf("%s does not end with %s", s.getDescription(), str.getDescription()),
	}

	args := []argI{s, str}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := s.getValue(s.v, msg)
		val2 := str.getValue(str.v, msg)
		return !strings.HasSuffix(val1, val2), nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (s *str) Contains(str *str) *Rule {
	id := fmt.Sprintf("s-c(%s,%s)", s.description, str.description)
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must contain %s", s.getDescription(), str.getDescription()),
		notRule:      fmt.Sprintf("%s must not contain %s", s.getDescription(), str.getDescription()),
		condition:    fmt.Sprintf("%s contains %s", s.getDescription(), str.getDescription()),
		notCondition: fmt.Sprintf("%s does not contain %s", s.getDescription(), str.getDescription()),
	}
	args := []argI{s, str}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := s.getValue(s.v, msg)
		val2 := str.getValue(str.v, msg)
		return !strings.Contains(val1, val2), nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

func (s *str) MatchesRegex(str *str) *Rule {
	id := fmt.Sprintf("s-mr(%s,%s)", s.description, str.description)
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must match regex %s", s.getDescription(), str.getDescription()),
		notRule:      fmt.Sprintf("%s must not match regex %s", s.getDescription(), str.getDescription()),
		condition:    fmt.Sprintf("%s matches regex %s", s.getDescription(), str.getDescription()),
		notCondition: fmt.Sprintf("%s does not match regex %s", s.getDescription(), str.getDescription()),
	}
	args := []argI{s, str}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		val1 := s.getValue(s.v, msg)
		val2 := str.getValue(str.v, msg)
		matched, err := regexp.MatchString(val2, val1)
		if err != nil {
			return false, err
		}
		return !matched, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
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
