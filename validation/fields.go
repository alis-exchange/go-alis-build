package validation

import (
	"fmt"
	"strings"
)

type Rule interface {
	Rule() string
	Satisfied() bool
	Fields() []string
	wrap()
	wrapped() bool
}

type Condition interface {
	Condition() string
	Satisfied() bool
	Fields() []string
	wrap()
}

type CustomRule struct {
	rule          string
	condition     string
	satisfiedFunc func() bool
	paths         []string
	isWrapped     bool
}

func customRule(fieldPaths []string, description string, satisfiedFunc func() bool) *CustomRule {
	return &CustomRule{
		paths:         fieldPaths,
		rule:          description,
		satisfiedFunc: satisfiedFunc,
	}
}

func (c *CustomRule) Rule() string {
	return c.rule
}

func (c *CustomRule) Condition() string {
	return c.condition
}

func (c *CustomRule) Satisfied() bool {
	return c.satisfiedFunc()
}

func (c *CustomRule) Fields() []string {
	return c.paths
}

func (c *CustomRule) wrap() {
	c.isWrapped = true
}

func (c *CustomRule) wrapped() bool {
	return c.isWrapped
}

type standard[T any] struct {
	rules      []string
	conditions []string
	satisfied  bool
	path       string
	value      T
	isWrapped  bool
}

func (s *standard[T]) Rule() string {
	if len(s.rules) == 0 {
		return fmt.Sprintf("%s must be a %T", s.path, s.value)
	} else {
		return fmt.Sprintf("%s must %s", s.path, strings.Join(s.rules, " and "))
	}
}

func (s *standard[T]) Condition() string {
	if len(s.conditions) == 0 {
		return fmt.Sprintf("%s is a %T", s.path, s.value)
	} else {
		return fmt.Sprintf("%s %s", s.path, strings.Join(s.conditions, " and "))
	}
}

func (s *standard[T]) Satisfied() bool {
	return s.satisfied
}

func (s *standard[T]) Fields() []string {
	return []string{s.path}
}

func (s *standard[T]) wrap() {
	s.isWrapped = true
}

func (s *standard[T]) wrapped() bool {
	return s.isWrapped
}

func newStandard[T any](path string, value T) standard[T] {
	return standard[T]{path: path, value: value, satisfied: true}
}

func (s *standard[T]) add(rule string, condition string, satisfied bool, args ...interface{}) {
	s.rules = append(s.rules, fmt.Sprintf(rule, args...))
	s.conditions = append(s.conditions, fmt.Sprintf(condition, args...))
	s.satisfied = s.satisfied && satisfied
}

type String struct {
	standard[string]
}

func newString(path string, value string) *String {
	return &String{newStandard(path, value)}
}

// func (s *String) Rule() string {
// 	if len(s.rules) == 0 {
// 		return fmt.Sprintf("%s must be a string", s.path)
// 	} else {
// 		return fmt.Sprintf("%s must %s", s.path, strings.Join(s.rules, " and "))
// 	}
// }

// func (s *String) Condition() string {
// 	if len(s.conditions) == 0 {
// 		return fmt.Sprintf("%s is a string", s.path)
// 	} else {
// 		return fmt.Sprintf("%s %s", s.path, strings.Join(s.conditions, " and "))
// 	}
// }

// func (s *String) Satisfied() bool {
// 	return s.satisfied
// }

// func (s *String) Fields() []string {
// 	return []string{s.path}
// }

// func (s *String) wrap() {
// 	s.isWrapped = true
// }

// func (s *String) wrapped() bool {
// 	return s.isWrapped
// }

func (s *String) Populated() *String {
	s.add("be populated", "is populated", s.value != "")
	return s
}

// func (s *StringField) StartsWith(prefix string) *StringField {
// 	s.addRule(fmt.Sprintf("start with %v", prefix), fmt.Sprintf("starts with %v", prefix), s.value == "" || s.value[:len(prefix)] == prefix)
// 	return s
// }

type Number[T interface {
	~int | ~int32 | ~int64 | ~float32 | ~float64
}] struct {
	standard[T]
}

func newNumber[T interface {
	~int | ~int32 | ~int64 | ~float32 | ~float64
}](path string, value T) *Number[T] {
	return &Number[T]{newStandard(path, value)}
}

func (n *Number[T]) Gt(min T) *Number[T] {
	n.add("be greater than %v", "is greater than %v", n.value > min, min)
	return n
}

// func numberField[T interface {
// 	~int | ~int32 | ~int64 | ~float32 | ~float64
// }](v *Validator, path string, value T) *NumberField[T] {
// 	return &NumberField[T]{v: v, path: path, value: value}
// }

// func (n *NumberField[T]) Gt(expected T) *NumberField[T] {
// 	// n.addRule(fmt.Sprintf("be greater than %v", expected), fmt.Sprintf("is greater than %v", expected), n.value > expected)
// 	return n
// }
