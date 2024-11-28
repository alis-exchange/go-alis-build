package validation

import (
	"fmt"
	"strings"
)

// Can be embedded in standard types to provide common methods needed to satisfy the Rule and Condition interfaces.
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

func (s *standard[T]) condition() string {
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

// Returns a standard instance, which can be embedded in standard types to provide common methods needed to satisfy the Rule and Condition interfaces.
func newStandard[T any](path string, value T) standard[T] {
	return standard[T]{path: path, value: value, satisfied: true}
}

func (s *standard[T]) add(rule string, condition string, satisfied bool, args ...interface{}) {
	s.rules = append(s.rules, fmt.Sprintf(rule, args...))
	s.conditions = append(s.conditions, fmt.Sprintf(condition, args...))
	s.satisfied = s.satisfied && satisfied
}
