package validation

import (
	"errors"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Holds the validation rules.
type Validator struct {
	// List of validation rules.
	rules []Rule
}

// Defines the methods that a validation rule must implement.
type Rule interface {
	// Returns the rule description.
	Rule() string
	// Checks if the rule is satisfied.
	Satisfied() bool
	// Returns the fields associated with the rule.
	Fields() []string
	// Marks the rule as wrapped.
	wrap()
	// Checks if the rule is wrapped.
	wrapped() bool
}

// Defines the methods that a condition must implement.
type Condition interface {
	// Returns the condition description.
	condition() string
	// Checks if the condition is satisfied.
	Satisfied() bool
	// Returns the fields associated with the condition.
	Fields() []string
	// Marks the condition as wrapped.
	wrap()
}

// Creates a new Validator instance.
func NewValidator() *Validator {
	return &Validator{}
}

// Returns all the rules that have been added.
func (v *Validator) Rules() []Rule {
	finalRules := []Rule{}
	for _, r := range v.rules {
		if r != nil && !r.wrapped() {
			finalRules = append(finalRules, r)
		}
	}
	return finalRules
}

// Returns the list of rules that are not satisfied.
func (v *Validator) BrokenRules() []Rule {
	broken := []Rule{}
	for _, r := range v.Rules() {
		if !r.Satisfied() {
			broken = append(broken, r)
		}
	}
	return broken
}

// Returns an error with human readable descriptions of all the broken rules, if any.
func (v *Validator) Validate() error {
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

// Adds a custom rule that is satisfied if any of the provided rules are satisfied.
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
	v.Custom(description, satisfied, paths...)
}

// Creates a ConditionalApplier that applies rules if all conditions are satisfied.
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
		descriptions = append(descriptions, c.condition())
		satisfied = satisfied && c.Satisfied()
	}
	description := strings.Join(descriptions, " and ")

	// return the conditional applier
	return &ConditionalApplier{v: v, description: description, satisfied: satisfied}
}

// Applies rules conditionally.
type ConditionalApplier struct {
	// Validator instance.
	v *Validator
	// Description of the condition.
	description string
	// Indicates if the condition is satisfied.
	satisfied bool
}

// Adds rules that are applied if the condition is satisfied.
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
	satisfied := true
	for _, r := range rules {
		paths = append(paths, r.Fields()...)
		ruleDescriptions = append(ruleDescriptions, r.Rule())
		if c.satisfied {
			satisfied = satisfied && r.Satisfied()
		}
	}
	description := "if " + c.description + ", " + strings.Join(ruleDescriptions, " and ")

	// add the rule
	c.v.Custom(description, satisfied, paths...)
}

// Defines a custom validation rule.
type CustomRule struct {
	// Rule description.
	rule string
	// Condition description.
	cond string
	// Function to check if the rule is satisfied.
	satisfiedFunc func() bool
	// Fields associated with the rule.
	paths []string
	// Indicates if the rule is wrapped.
	isWrapped bool
}

// Returns the rule description.
func (c *CustomRule) Rule() string {
	return c.rule
}

// Returns the condition description.
func (c *CustomRule) condition() string {
	return c.cond
}

// Checks if the rule is satisfied.
func (c *CustomRule) Satisfied() bool {
	return c.satisfiedFunc()
}

// Returns the fields associated with the rule.
func (c *CustomRule) Fields() []string {
	return c.paths
}

// Marks the rule as wrapped.
func (c *CustomRule) wrap() {
	c.isWrapped = true
}

// Checks if the rule is wrapped.
func (c *CustomRule) wrapped() bool {
	return c.isWrapped
}

// Adds a custom rule with a static satisfaction status.
func (v *Validator) Custom(description string, satisfied bool, paths ...string) *CustomRule {
	return v.CustomEvaluated(description, func() bool { return satisfied }, paths...)
}

// Adds a custom rule with a dynamic satisfaction status.
func (v *Validator) CustomEvaluated(description string, satisfiedFunc func() bool, paths ...string) *CustomRule {
	rule := &CustomRule{
		rule:          description,
		cond:          description,
		satisfiedFunc: satisfiedFunc,
		paths:         paths,
	}
	v.rules = append(v.rules, rule)
	return rule
}

// Returns a temporary object for creating rules on a string field.
func (v *Validator) String(path, value string) *String {
	r := &String{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an int field.
func (v *Validator) Int(path string, value int) *Number[int] {
	r := &Number[int]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an int8 field.
func (v *Validator) Int8(path string, value int8) *Number[int8] {
	r := &Number[int8]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an int16 field.
func (v *Validator) Int16(path string, value int16) *Number[int16] {
	r := &Number[int16]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an int32 field.
func (v *Validator) Int32(path string, value int32) *Number[int32] {
	r := &Number[int32]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an int64 field.
func (v *Validator) Int64(path string, value int64) *Number[int64] {
	r := &Number[int64]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a float32 field.
func (v *Validator) Float32(path string, value float32) *Number[float32] {
	r := &Number[float32]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a float64 field.
func (v *Validator) Float64(path string, value float64) *Number[float64] {
	r := &Number[float64]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a uint field.
func (v *Validator) Uint(path string, value uint) *Number[uint] {
	r := &Number[uint]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a uint8 field.
func (v *Validator) Uint8(path string, value uint8) *Number[uint8] {
	r := &Number[uint8]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a uint16 field.
func (v *Validator) Uint16(path string, value uint16) *Number[uint16] {
	r := &Number[uint16]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a uint32 field.
func (v *Validator) Uint32(path string, value uint32) *Number[uint32] {
	r := &Number[uint32]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a uint64 field.
func (v *Validator) Uint64(path string, value uint64) *Number[uint64] {
	r := &Number[uint64]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a bool field.
func (v *Validator) Bool(path string, value bool) *Bool {
	r := &Bool{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a string list field.
func (v *Validator) StringList(path string, value []string) *StringList {
	r := &StringList{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an int list field.
func (v *Validator) IntList(path string, value []int) *NumberList[int] {
	r := &NumberList[int]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an int8 list field.
func (v *Validator) Int8List(path string, value []int8) *NumberList[int8] {
	r := &NumberList[int8]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an int16 list field.
func (v *Validator) Int16List(path string, value []int16) *NumberList[int16] {
	r := &NumberList[int16]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an int32 list field.
func (v *Validator) Int32List(path string, value []int32) *NumberList[int32] {
	r := &NumberList[int32]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an int64 list field.
func (v *Validator) Int64List(path string, value []int64) *NumberList[int64] {
	r := &NumberList[int64]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a float32 list field.
func (v *Validator) Float32List(path string, value []float32) *NumberList[float32] {
	r := &NumberList[float32]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a float64 list field.
func (v *Validator) Float64List(path string, value []float64) *NumberList[float64] {
	r := &NumberList[float64]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a uint list field.
func (v *Validator) UintList(path string, value []uint) *NumberList[uint] {
	r := &NumberList[uint]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a uint8 list field.
func (v *Validator) Uint8List(path string, value []uint8) *NumberList[uint8] {
	r := &NumberList[uint8]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a uint16 list field.
func (v *Validator) Uint16List(path string, value []uint16) *NumberList[uint16] {
	r := &NumberList[uint16]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a uint32 list field.
func (v *Validator) Uint32List(path string, value []uint32) *NumberList[uint32] {
	r := &NumberList[uint32]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a uint64 list field.
func (v *Validator) Uint64List(path string, value []uint64) *NumberList[uint64] {
	r := &NumberList[uint64]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on an enum field.
func (v *Validator) Enum(path string, value protoreflect.Enum) *Enum {
	r := &Enum{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a timestamp field.
func (v *Validator) Timestamp(path string, value *timestamppb.Timestamp) *Timestamp {
	r := &Timestamp{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a duration field.
func (v *Validator) Duration(path string, value *durationpb.Duration) *Duration {
	r := &Duration{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Returns a temporary object for creating rules on a message field.
func (v *Validator) MessageIsPopulated(path string, isPopulated bool) *CustomRule {
	return v.Custom(path+" must be populated", isPopulated, path)
}

// Returns a temporary object for creating rules on a message field.
func (v *Validator) EachMessagePopulated(path string, isPopulated bool) *CustomRule {
	return v.Custom(path+" must have all values populated", isPopulated, path)
}
