package validation

import (
	"errors"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Validator holds the validation rules and provides methods to add new rules.
// It is the main entry point for creating and executing validation logic.
type Validator struct {
	// rules is the list of validation rules to be checked.
	rules []Rule
}

// Rule defines the methods that a validation rule must implement.
// All specific rule types (e.g., String, Number, CustomRule) implement this interface.
type Rule interface {
	// Rule returns the human-readable description of the rule.
	Rule() string
	// Satisfied checks if the rule is satisfied.
	Satisfied() bool
	// Fields returns the list of field paths associated with the rule.
	Fields() []string
	// wrap marks the rule as being part of a larger logical block (like a condition or an OR group).
	wrap()
	// wrapped returns whether the rule has been wrapped.
	wrapped() bool
}

// Condition defines the methods that a condition must implement.
// Conditions are used in conditional validation logic (e.g., If(...)).
type Condition interface {
	// condition returns the human-readable description of the condition.
	condition() string
	// Satisfied checks if the condition is met.
	Satisfied() bool
	// Fields returns the list of field paths associated with the condition.
	Fields() []string
	// wrap marks the condition as being part of a larger logical block.
	wrap()
}

/*
NewValidator creates and returns a new Validator instance.

The Validator is used to chain validation rules for various data types.
Once all rules are added, call Validate() to check them.

Example:

	v := validation.NewValidator()
	v.String("name", req.GetName()).IsPopulated()
	v.Int32("age", req.GetAge()).Gt(18)
	v.If(v.Enum("status", req.GetStatus()).Is(validation.User_ACTIVE)).Then(
		v.Timestamp("update_time", req.GetUpdateTime()).NotInFuture(),
	)
	v.Custom("name must be populated", req.GetName() != "")
	v.Custom("age must be greater than 18", req.GetAge() > 18)
	v.CustomEvaluated("if status is ACTIVE, update time must not be in the future", func() bool {
		if req.GetStatus() == validation.User_ACTIVE {
			return req.GetUpdateTime().AsTime().Before(time.Now())
		}
		return true
	})

	// validate
	err := v.Validate()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "%s", err.Error())
	}
*/
func NewValidator() *Validator {
	return &Validator{}
}

// Rules returns all the rules that have been added to the validator.
// This includes both satisfied and unsatisfied rules.
func (v *Validator) Rules() []Rule {
	finalRules := []Rule{}
	for _, r := range v.rules {
		if r != nil && !r.wrapped() {
			finalRules = append(finalRules, r)
		}
	}
	return finalRules
}

// BrokenRules returns the list of rules that are NOT satisfied.
func (v *Validator) BrokenRules() []Rule {
	broken := []Rule{}
	for _, r := range v.Rules() {
		if !r.Satisfied() {
			broken = append(broken, r)
		}
	}
	return broken
}

// Validate checks all the added rules and returns an error if any rule is broken.
// The error message contains human-readable descriptions of all the broken rules.
// Returns nil if all rules are satisfied.
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

// Or adds a composite rule that is satisfied if ANY of the provided rules are satisfied.
// This is useful for logical OR conditions (e.g., "email OR phone number must be provided").
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

// If creates a conditional validation block.
// The subsequent rules added via .Then() will only be evaluated if all the provided conditions are satisfied.
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

// ConditionalApplier is a helper struct for building conditional validation rules.
// It is created by Validator.If().
type ConditionalApplier struct {
	// Validator instance.
	v *Validator
	// Description of the condition.
	description string
	// Indicates if the condition is satisfied.
	satisfied bool
}

// Then adds rules that are only applicable if the condition (defined in .If()) is satisfied.
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

// CustomRule defines a custom validation rule with a dynamic or static satisfaction check.
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

// Rule returns the description of the custom rule.
func (c *CustomRule) Rule() string {
	return c.rule
}

// condition returns the description of the condition associated with the rule.
func (c *CustomRule) condition() string {
	return c.cond
}

// Satisfied checks if the custom rule is satisfied by executing the provided function.
func (c *CustomRule) Satisfied() bool {
	return c.satisfiedFunc()
}

// Fields returns the list of field paths associated with the custom rule.
func (c *CustomRule) Fields() []string {
	return c.paths
}

// wrap marks the custom rule as wrapped (part of a larger logical block).
func (c *CustomRule) wrap() {
	c.isWrapped = true
}

// wrapped returns whether the custom rule is wrapped.
func (c *CustomRule) wrapped() bool {
	return c.isWrapped
}

// Custom adds a custom rule with a static satisfaction status.
// description: A human-readable description of the rule.
// satisfied: Whether the rule is currently satisfied.
// paths: (Optional) Field paths associated with this rule.
func (v *Validator) Custom(description string, satisfied bool, paths ...string) *CustomRule {
	return v.CustomEvaluated(description, func() bool { return satisfied }, paths...)
}

// CustomEvaluated adds a custom rule where satisfaction is determined by a function.
// description: A human-readable description of the rule.
// satisfiedFunc: A function that returns true if the rule is satisfied.
// paths: (Optional) Field paths associated with this rule.
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

// String creates a new validation rule for a string field.
func (v *Validator) String(path, value string) *String {
	r := &String{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Int creates a new validation rule for an int field.
func (v *Validator) Int(path string, value int) *Number[int] {
	r := &Number[int]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Int8 creates a new validation rule for an int8 field.
func (v *Validator) Int8(path string, value int8) *Number[int8] {
	r := &Number[int8]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Int16 creates a new validation rule for an int16 field.
func (v *Validator) Int16(path string, value int16) *Number[int16] {
	r := &Number[int16]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Int32 creates a new validation rule for an int32 field.
func (v *Validator) Int32(path string, value int32) *Number[int32] {
	r := &Number[int32]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Int64 creates a new validation rule for an int64 field.
func (v *Validator) Int64(path string, value int64) *Number[int64] {
	r := &Number[int64]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Float32 creates a new validation rule for a float32 field.
func (v *Validator) Float32(path string, value float32) *Number[float32] {
	r := &Number[float32]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Float64 creates a new validation rule for a float64 field.
func (v *Validator) Float64(path string, value float64) *Number[float64] {
	r := &Number[float64]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Uint creates a new validation rule for a uint field.
func (v *Validator) Uint(path string, value uint) *Number[uint] {
	r := &Number[uint]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Uint8 creates a new validation rule for a uint8 field.
func (v *Validator) Uint8(path string, value uint8) *Number[uint8] {
	r := &Number[uint8]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Uint16 creates a new validation rule for a uint16 field.
func (v *Validator) Uint16(path string, value uint16) *Number[uint16] {
	r := &Number[uint16]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Uint32 creates a new validation rule for a uint32 field.
func (v *Validator) Uint32(path string, value uint32) *Number[uint32] {
	r := &Number[uint32]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Uint64 creates a new validation rule for a uint64 field.
func (v *Validator) Uint64(path string, value uint64) *Number[uint64] {
	r := &Number[uint64]{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Bool creates a new validation rule for a bool field.
func (v *Validator) Bool(path string, value bool) *Bool {
	r := &Bool{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// StringList creates a new validation rule for a list of strings.
func (v *Validator) StringList(path string, value []string) *StringList {
	r := &StringList{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// IntList creates a new validation rule for a list of ints.
func (v *Validator) IntList(path string, value []int) *NumberList[int] {
	r := &NumberList[int]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Int8List creates a new validation rule for a list of int8s.
func (v *Validator) Int8List(path string, value []int8) *NumberList[int8] {
	r := &NumberList[int8]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Int16List creates a new validation rule for a list of int16s.
func (v *Validator) Int16List(path string, value []int16) *NumberList[int16] {
	r := &NumberList[int16]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Int32List creates a new validation rule for a list of int32s.
func (v *Validator) Int32List(path string, value []int32) *NumberList[int32] {
	r := &NumberList[int32]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Int64List creates a new validation rule for a list of int64s.
func (v *Validator) Int64List(path string, value []int64) *NumberList[int64] {
	r := &NumberList[int64]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Float32List creates a new validation rule for a list of float32s.
func (v *Validator) Float32List(path string, value []float32) *NumberList[float32] {
	r := &NumberList[float32]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Float64List creates a new validation rule for a list of float64s.
func (v *Validator) Float64List(path string, value []float64) *NumberList[float64] {
	r := &NumberList[float64]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// UintList creates a new validation rule for a list of uints.
func (v *Validator) UintList(path string, value []uint) *NumberList[uint] {
	r := &NumberList[uint]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Uint8List creates a new validation rule for a list of uint8s.
func (v *Validator) Uint8List(path string, value []uint8) *NumberList[uint8] {
	r := &NumberList[uint8]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Uint16List creates a new validation rule for a list of uint16s.
func (v *Validator) Uint16List(path string, value []uint16) *NumberList[uint16] {
	r := &NumberList[uint16]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Uint32List creates a new validation rule for a list of uint32s.
func (v *Validator) Uint32List(path string, value []uint32) *NumberList[uint32] {
	r := &NumberList[uint32]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Uint64List creates a new validation rule for a list of uint64s.
func (v *Validator) Uint64List(path string, value []uint64) *NumberList[uint64] {
	r := &NumberList[uint64]{newList(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Enum creates a new validation rule for an enum field.
func (v *Validator) Enum(path string, value protoreflect.Enum) *Enum {
	r := &Enum{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Timestamp creates a new validation rule for a timestamp field.
func (v *Validator) Timestamp(path string, value *timestamppb.Timestamp) *Timestamp {
	r := &Timestamp{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// Duration creates a new validation rule for a duration field.
func (v *Validator) Duration(path string, value *durationpb.Duration) *Duration {
	r := &Duration{newStandard(path, value)}
	v.rules = append(v.rules, r)
	return r
}

// MessageIsPopulated adds a rule that checks if a message field is populated.
// This is typically used for checking if a sub-message (e.g., "address") is present.
func (v *Validator) MessageIsPopulated(path string, isPopulated bool) *CustomRule {
	return v.Custom(path+" must be populated", isPopulated, path)
}

// EachMessagePopulated adds a rule that checks if each message in a list is populated.
// This is typically used for checking if all elements in a repeated message field are valid/present.
func (v *Validator) EachMessagePopulated(path string, isPopulated bool) *CustomRule {
	return v.Custom(path+" must have all values populated", isPopulated, path)
}

// SetIfNil returns the fallback value if the pointer is nil, otherwise returns the pointer itself.
// This is a helper function for handling optional fields.
func SetIfNil[T any](pointer *T, fallback *T) *T {
	if pointer == nil {
		return fallback
	}
	return pointer
}
