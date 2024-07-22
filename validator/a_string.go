package validator

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.alis.build/alog"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type str struct {
	repeated    bool
	paths       []string
	getValues   func(v *Validator, msg protoreflect.ProtoMessage) []string
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
	s.getValues(v, v.protoMsg)
	s.v = v
}

// Fixed string value
func String(val string) *str {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []string {
		return []string{val}
	}
	return &str{description: fmt.Sprintf("'%s'", val), getValues: getValuesFunc}
}

// String field
func StringField(path string) *str {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []string {
		return []string{v.getString(msg, path)}
	}
	return &str{description: path, getValues: getValuesFunc, paths: []string{path}}
}

// String list field
func EachStringIn(path string) *str {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []string {
		return v.getStringList(msg, path)
	}
	return &str{description: fmt.Sprintf("each string in %s", path), getValues: getValuesFunc, paths: []string{path}, repeated: true}
}

// Rule that ensures s is populated
func (s *str) Populated() *Rule {
	id := fmt.Sprintf("s-pop(%s)", s.description)
	descr := &Descriptions{
		rule:         fmt.Sprintf("%s must be populated", s.getDescription()),
		notRule:      fmt.Sprintf("%s must not be populated", s.getDescription()),
		condition:    fmt.Sprintf("%s is populated", s.getDescription()),
		notCondition: fmt.Sprintf("%s is not populated", s.getDescription()),
	}
	args := []argI{s}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		for _, val := range s.getValues(s.v, msg) {
			if val == "" {
				return true, nil
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Rule that ensures s is equal to s2
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
		for _, val1 := range s.getValues(s.v, msg) {
			for _, val2 := range str.getValues(str.v, msg) {
				if val1 != val2 {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Rule that ensures s starts with s2
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
		for _, val1 := range s.getValues(s.v, msg) {
			for _, val2 := range str.getValues(str.v, msg) {
				if !strings.HasPrefix(val1, val2) {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Rule that ensures s ends with s2
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
		for _, val1 := range s.getValues(s.v, msg) {
			for _, val2 := range str.getValues(str.v, msg) {
				if !strings.HasSuffix(val1, val2) {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Rule that ensures s contains s2
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
		for _, val1 := range s.getValues(s.v, msg) {
			for _, val2 := range str.getValues(str.v, msg) {
				if !strings.Contains(val1, val2) {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Rule that ensures s matches regex str
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
		for _, val1 := range s.getValues(s.v, msg) {
			for _, val2 := range str.getValues(str.v, msg) {
				matched, err := regexp.MatchString(val2, val1)
				if err != nil {
					return false, err
				}
				if !matched {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Returns the length of s
func (s *str) Length() *integer {
	if s.repeated {
		alog.Fatalf(context.Background(), "Length() is not supported for EachStringIn fields")
	}
	description := fmt.Sprintf("length of %s", s.getDescription())
	newI := &integer{description: description, paths: s.paths}
	newI.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []int64 {
		return []int64{int64(len(s.getValues(v, msg)[0]))}
	}
	return newI
}

// Returns the length of s as a float
func (s *str) LengthAsFloat() *float {
	if s.repeated {
		alog.Fatalf(context.Background(), "LengthAsFloat() is not supported for EachStringIn fields")
	}
	description := fmt.Sprintf("length of %s", s.getDescription())
	newF := &float{description: description, paths: s.paths}
	newF.getValues = func(v *Validator, msg protoreflect.ProtoMessage) []float64 {
		return []float64{float64(len(s.getValues(v, msg)[0]))}
	}
	return newF
}
