package validation

import "fmt"

type Field interface {
	rule() *Rule
	condition() *Rule
}

func updateRule(v *Validator, rule *Rule, path string, description string, satisfied bool) *Rule {
	if rule == nil {
		rule = &Rule{
			satisfied:   satisfied,
			description: path + " " + description,
		}
		if v != nil {
			v.rules = append(v.rules, rule)
		}
	} else {
		rule.description += " and " + description
		rule.satisfied = rule.satisfied && satisfied
	}
	return rule
}

func updateCondition(condition *Rule, path string, description string, satisfied bool) *Rule {
	return updateRule(nil, condition, path, description, satisfied)
}

type StringField struct {
	v     *Validator
	path  string
	value string
	r     *Rule
	c     *Rule
}

func stringField(v *Validator, path string, value string) *StringField {
	return &StringField{v: v, path: path, value: value}
}

func (s *StringField) addRule(rule string, condition string, satisfied bool) {
	s.r = updateRule(s.v, s.r, s.path, rule, satisfied)
	s.c = updateCondition(s.c, s.path, condition, satisfied)
}

func (s *StringField) rule() *Rule {
	s.r.wrapped = true
	return s.r
}

func (s *StringField) condition() *Rule {
	s.r.wrapped = true
	return s.c
}

func (s *StringField) Populated() *StringField {
	s.addRule("must be populated", "is populated", s.value != "")
	return s
}

type NumberField[T interface {
	~int | ~int32 | ~int64 | ~float32 | ~float64
}] struct {
	v     *Validator
	path  string
	value T
	r     *Rule
	c     *Rule
}

func numberField[T interface {
	~int | ~int32 | ~int64 | ~float32 | ~float64
}](v *Validator, path string, value T) *NumberField[T] {
	return &NumberField[T]{v: v, path: path, value: value}
}

func (n *NumberField[T]) addRule(rule string, condition string, satisfied bool) {
	n.r = updateRule(n.v, n.r, n.path, rule, satisfied)
	n.c = updateCondition(n.c, n.path, condition, satisfied)
}

func (n *NumberField[T]) rule() *Rule {
	n.r.wrapped = true
	return n.r
}

func (n *NumberField[T]) condition() *Rule {
	n.r.wrapped = true
	return n.c
}

func (n *NumberField[T]) Gt(expected T) *NumberField[T] {
	n.addRule(fmt.Sprintf("must be greater than %v", expected), fmt.Sprintf("is greater than %v", expected), n.value > expected)
	return n
}
