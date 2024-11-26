package validation

import (
	"errors"
	"strings"
)

type Rule struct {
	description   string
	satisfiedFunc func() bool
	satisfied     bool
	condition     *Condition
	wrapped       bool
}

type Condition struct {
	v           *Validator
	description string
	satisfied   bool
}

func (c *Condition) Then(field Field, fields ...Field) {
	rule := &Rule{satisfied: field.rule().satisfied, description: field.rule().description}
	for _, f := range fields {
		rule.satisfied = rule.satisfied && f.rule().satisfied
		rule.description += " and " + f.rule().description
	}
	rule.condition = c
	c.v.rules = append(c.v.rules, rule)
}

// Returns whether the rule is satisfied
func (r *Rule) Satisfied() bool {
	// if rule was combined/wrapped it would have been made nil, so as not to be evaluated
	if r == nil || r.wrapped {
		return true
	}

	// rule is satisfied if the condition for its application is not met
	if r.condition != nil && !r.condition.satisfied {
		return true
	}

	// evaluate the rule if it hasn't been evaluated yet
	if r.satisfiedFunc != nil {
		r.satisfied = r.satisfiedFunc()
		r.satisfiedFunc = nil
	}
	return r.satisfied
}

func (r *Rule) Description() string {
	if r == nil || r.wrapped {
		return ""
	}

	descr := r.description
	if r.condition != nil {
		descr = "if " + r.condition.description + ", " + descr
	}
	return descr
}

type Validator struct {
	rules []*Rule
}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) Rules() []*Rule {
	finalRules := []*Rule{}
	for _, r := range v.rules {
		if r != nil && !r.wrapped {
			finalRules = append(finalRules, r)
		}
	}
	return finalRules
}

func (v *Validator) BrokenRules() []*Rule {
	broken := []*Rule{}
	for _, r := range v.Rules() {
		if !r.Satisfied() {
			broken = append(broken, r)
		}
	}
	return broken
}

func (v *Validator) Error() error {
	broken := v.BrokenRules()
	errDescriptions := []string{}
	for _, r := range broken {
		errDescriptions = append(errDescriptions, r.Description())
	}
	if len(errDescriptions) == 0 {
		return nil
	}
	return errors.New(strings.Join(errDescriptions, "; "))
}

func (v *Validator) String(path, value string) *StringField {
	return stringField(v, path, value)
}

func (v *Validator) Int32(path string, value int32) *NumberField[int32] {
	return numberField(v, path, value)
}

func (v *Validator) Int64(path string, value int64) *NumberField[int64] {
	return numberField(v, path, value)
}

func (v *Validator) Float32(path string, value float32) *NumberField[float32] {
	return numberField(v, path, value)
}

func (v *Validator) Float64(path string, value float64) *NumberField[float64] {
	return numberField(v, path, value)
}

func (v *Validator) If(field Field, fields ...Field) *Condition {
	condition := &Condition{v: v, description: field.condition().description, satisfied: field.condition().Satisfied()}
	for _, f := range fields {
		condition.satisfied = condition.satisfied && f.condition().Satisfied()
		condition.description += " and " + f.condition().description
	}
	return condition
}

func (v *Validator) Or(field Field, fields ...Field) {
	rule := &Rule{satisfied: field.rule().satisfied, description: field.rule().description}
	for _, f := range fields {
		rule.satisfied = rule.satisfied || f.rule().satisfied
		rule.description += " or " + f.rule().description
	}
	v.rules = append(v.rules, rule)
}
