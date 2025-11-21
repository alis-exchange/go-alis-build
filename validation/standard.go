package validation

import (
	"fmt"
	"strings"
)

// standard is a generic struct that implements common methods required by the Rule and Condition interfaces.
// It can be embedded in type-specific rule structs (like String, Int, etc.) to avoid code duplication.
type standard[T any] struct {
	rules      []string
	conditions []string
	satisfied  bool
	path       string
	value      T
	isWrapped  bool
}

// Rule constructs and returns a human-readable description of the validation rule(s).
// If multiple internal rules are present, they are joined with "and".
func (s *standard[T]) Rule() string {
	if len(s.rules) == 0 {
		return fmt.Sprintf("%s must be a %T", s.path, s.value)
	} else {
		return fmt.Sprintf("%s must %s", s.path, strings.Join(s.rules, " and "))
	}
}

// condition constructs and returns a human-readable description of the condition(s).
// This is used when the rule is part of a conditional expression (e.g. If(...)).
func (s *standard[T]) condition() string {
	if len(s.conditions) == 0 {
		return fmt.Sprintf("%s is a %T", s.path, s.value)
	} else {
		return fmt.Sprintf("%s %s", s.path, strings.Join(s.conditions, " and "))
	}
}

// Satisfied returns true if all the accumulated rules/checks in this instance are satisfied.
func (s *standard[T]) Satisfied() bool {
	return s.satisfied
}

// Fields returns the list of field paths involved in this rule.
// For a standard rule, this is typically just the single field path it operates on.
func (s *standard[T]) Fields() []string {
	return []string{s.path}
}

// wrap marks this rule instance as being part of a larger logical block (like an Or group or conditional).
// When wrapped, the individual rule is not added directly to the validator's top-level rule list.
func (s *standard[T]) wrap() {
	s.isWrapped = true
}

// wrapped returns true if this rule instance has been wrapped.
func (s *standard[T]) wrapped() bool {
	return s.isWrapped
}

// newStandard creates and returns a new initialized standard instance.
func newStandard[T any](path string, value T) standard[T] {
	return standard[T]{path: path, value: value, satisfied: true}
}

// add appends a new rule/check to the standard instance.
// rule: format string for the rule description (e.g., "be positive").
// condition: format string for the condition description (e.g., "is positive").
// satisfied: boolean result of the check.
// args: arguments for the format strings.
func (s *standard[T]) add(rule string, condition string, satisfied bool, args ...interface{}) {
	s.rules = append(s.rules, fmt.Sprintf(rule, args...))
	s.conditions = append(s.conditions, fmt.Sprintf(condition, args...))
	s.satisfied = s.satisfied && satisfied
}
