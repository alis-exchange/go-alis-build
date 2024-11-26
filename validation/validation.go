package validation

import (
	"context"
	"errors"
	"strings"
)

type Validator struct {
	ctx   context.Context
	rules []*Rule
}

type Condition struct {
	satisfied   bool
	description string
}

type Rule struct {
	description   string
	satisfiedFunc func() bool
	dependsOn     []*Rule
	satisfied     bool
	// only apply rule if condition is met
	condition *Condition
}

// Options for values in rules
type ValueOptions struct {
	valueReferences []string
}

// A functional option to set the value options
type ValueOption func(*ValueOptions)

// Use if one/more of the values provided for the rule are not constant but retrieved from other paths in the struct.
// Will update the rule description to include the paths of the values.
// E.g. "'create_time' must be before 'update_time'"
func WithReferencedValues(paths ...string) ValueOption {
	return func(o *ValueOptions) {
		o.valueReferences = paths
	}
}

// Adds one/more dependencies to the rule, which if not satisfied, will cause the rule to also be unsatisfied.
// The rule itself is not evaluated if any of its dependencies are not satisfied.
func (r *Rule) DependsOn(rules ...*Rule) {
	r.dependsOn = rules
}

// Returns whether the rule is satisfied
func (r *Rule) Satisfied() bool {
	// rule is not satisfied if any of its dependencies are not satisfied
	for _, d := range r.dependsOn {
		if !d.Satisfied() {
			return false
		}
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
	descr := r.description
	if r.condition != nil {
		descr = "if " + r.condition.description + ", " + descr
	}
	return descr
}

func NewValidator(ctx context.Context) *Validator {
	return &Validator{
		ctx: ctx,
	}
}

func (v *Validator) AddRule(description string, satisfied bool) *Validator {
	r := &Rule{
		description: description,
		satisfied:   satisfied,
	}
	v.rules = append(v.rules, r)
	return v
}

func (v *Validator) AddEvaluatedRule(description string, satisfiedFunc func() bool) *Validator {
	r := &Rule{
		description:   description,
		satisfiedFunc: satisfiedFunc,
	}
	v.rules = append(v.rules, r)
	return v
}

type ErrorOptions struct {
	returnFirst bool
}

type ErrorOption func(*ErrorOptions)

func ReturnFirstError() ErrorOption {
	return func(o *ErrorOptions) {
		o.returnFirst = true
	}
}

func (v *Validator) Validate(opts ...ErrorOption) error {
	options := &ErrorOptions{}
	for _, o := range opts {
		o(options)
	}
	errs := []string{}
	for _, r := range v.rules {
		if !r.Satisfied() {
			errs = append(errs, r.Description())
			if !options.returnFirst {
				break
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
}

func (v *Validator) Rules() []*Rule {
	return v.rules
}

func (v *Validator) BrokenRules() []*Rule {
	broken := []*Rule{}
	for _, r := range v.rules {
		if !r.Satisfied() {
			broken = append(broken, r)
		}
	}
	return broken
}
