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

type Rule struct {
	Description     string
	satisfied       func() bool
	dependsOn       []*Rule
	combo           bool
	evaluated       bool
	satisfiedResult bool
}

func (r *Rule) DependsOn(rules ...*Rule) {
	r.dependsOn = rules
}

func (r *Rule) Satisfied() bool {
	if !r.evaluated {
		r.satisfiedResult = r.satisfied()
	}
	for _, d := range r.dependsOn {
		if !d.Satisfied() {
			return false
		}
	}
	return r.satisfiedResult
}

func NewValidator(ctx context.Context) *Validator {
	return &Validator{
		ctx: ctx,
	}
}

type IntField struct {
	path  string
	value int
}

func (v *Validator) InRange(path string, value float64, min float64, max float64) *Rule {
	r := &Rule{
		satisfied: func() bool {
			return value >= min && value <= max
		},
	}
	return r
}

func (v *Validator) OR(rules ...*Rule) *Rule {
	satisfiedFunc := func() bool {
		for _, r := range rules {
			if r.Satisfied() {
				return true
			}
		}
		return false
	}
	descriptions := []string{}
	for _, r := range rules {
		descriptions = append(descriptions, r.Description)
	}
	r := &Rule{
		satisfied:   satisfiedFunc,
		Description: strings.Join(descriptions, " OR "),
		combo:       true,
	}
	return r
}

func (v *Validator) AND(rules ...*Rule) *Rule {
	satisfiedFunc := func() bool {
		for _, r := range rules {
			if !r.Satisfied() {
				return false
			}
		}
		return true
	}
	descriptions := []string{}
	for _, r := range rules {
		descrToAdd := r.Description
		if r.combo {
			descrToAdd = "(" + descrToAdd + ")"
		}
		descriptions = append(descriptions, descrToAdd)
	}
	r := &Rule{
		satisfied:   satisfiedFunc,
		Description: strings.Join(descriptions, " AND "),
		combo:       true,
	}
	return r
}

func (v *Validator) AddBasicRule(description string, satisfied bool) *Validator {
	r := &Rule{
		Description: description,
		satisfied:   func() bool { return satisfied },
	}
	v.rules = append(v.rules, r)
	return v
}

func (v *Validator) AddEvaluatedRule(description string, satisfied func() bool) *Validator {
	r := &Rule{
		Description: description,
		satisfied:   satisfied,
	}
	v.rules = append(v.rules, r)
	return v
}

func (v *Validator) AddRule(rule *Rule) *Validator {
	v.rules = append(v.rules, rule)
	return v
}

type ErrorOptions struct {
	returnAll bool
}

type ErrorOption func(*ErrorOptions)

func WithReturnFirstError() ErrorOption {
	return func(o *ErrorOptions) {
		o.returnAll = true
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
			errs = append(errs, r.Description)
			if !options.returnAll {
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

func (v *Validator) SF(path string, valueFn func() string) *StringField {
	return v.StringField(path, valueFn)
}

func (v *Validator) StringField(path string, valueFn func() string) *StringField {
	return &StringField{
		path:  path,
		value: valueFn,
	}
}

type SF struct {
	path  string
	value string
}

type Rgx interface{}

func Matches(rgx Rgx)
