package validation

import (
	"errors"
	"strings"
)

type Validator struct {
	rules []Rule
}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) Rules() []Rule {
	finalRules := []Rule{}
	for _, r := range v.rules {
		if r != nil && !r.wrapped() {
			finalRules = append(finalRules, r)
		}
	}
	return finalRules
}

func (v *Validator) BrokenRules() []Rule {
	broken := []Rule{}
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
		errDescriptions = append(errDescriptions, r.Rule())
	}
	if len(errDescriptions) == 0 {
		return nil
	}
	return errors.New(strings.Join(errDescriptions, "; "))
}

func (v *Validator) Or(rules ...Rule) {
	// stop if rules are nil
	if len(rules) == 0 {
		return
	}
	// wrap all the provided rules
	for _, r := range rules {
		r.wrap()
	}

	// setup paths, descriptions, and satisfied
	paths := []string{}
	ruleDescriptions := []string{}
	satisfied := false
	for _, r := range rules {
		paths = append(paths, r.Fields()...)
		ruleDescriptions = append(ruleDescriptions, r.Rule())
		satisfied = satisfied || r.Satisfied()
	}
	description := "either " + strings.Join(ruleDescriptions, " or ")

	// add the rule
	v.Custom(paths, description, satisfied)
}

func (v *Validator) If(conditions ...Condition) *ConditionalApplier {
	if len(conditions) == 0 {
		return nil
	}
	// wrap all the provided conditions
	for _, c := range conditions {
		c.wrap()
	}

	// setup description and satisfied
	descriptions := []string{}
	satisfied := true
	for _, c := range conditions {
		descriptions = append(descriptions, c.Condition())
		satisfied = satisfied && c.Satisfied()
	}
	description := strings.Join(descriptions, " and ")

	// return the conditional applier
	return &ConditionalApplier{v: v, description: description, satisfied: satisfied}
}

type ConditionalApplier struct {
	v           *Validator
	description string
	satisfied   bool
}

func (c *ConditionalApplier) Then(rules ...Rule) {
	// wrap all the provided rules
	for _, r := range rules {
		r.wrap()
	}

	// stop if c or rules are nil
	if c == nil {
		return
	}
	if len(rules) == 0 {
		return
	}

	// setup paths, descriptions, and satisfied
	paths := []string{}
	ruleDescriptions := []string{}
	satisfied := !c.satisfied
	for _, r := range rules {
		paths = append(paths, r.Fields()...)
		ruleDescriptions = append(ruleDescriptions, r.Rule())
		satisfied = satisfied && r.Satisfied()
	}
	description := "if " + c.description + ", " + strings.Join(ruleDescriptions, " and ")

	// add the rule
	c.v.Custom(paths, description, satisfied)
}

func (v *Validator) Custom(fieldPaths []string, description string, satisfied bool) *CustomRule {
	rule := customRule(fieldPaths, description, func() bool { return satisfied })
	v.rules = append(v.rules, rule)
	return rule
}

func (v *Validator) CustomEvaluated(fieldPaths []string, description string, satisfiedFunc func() bool) *CustomRule {
	rule := customRule(fieldPaths, description, satisfiedFunc)
	v.rules = append(v.rules, rule)
	return rule
}

func (v *Validator) String(path, value string) *String {
	r := newString(path, value)
	v.rules = append(v.rules, r)
	return r
}

func (v *Validator) Int32(path string, value int32) *Number[int32] {
	r := newNumber(path, value)
	v.rules = append(v.rules, r)
	return r
}
