package validation

import (
	"fmt"
)

// Require a primitive (string, number, bool) value at a given path to be equal to a given value.
//   - path: the path in the struct to check (e.g. "name" or "display_name")
//   - value: the value to compare against
//   - expected: the expected value
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) Eq(path string, value, expected any) *Validator {
	v.EqR(path, value, expected)
	return v
}

// Does the same as Eq, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) EqR(path string, value, actual any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must be equal to %v", path, value),
		satisfied:   value == actual,
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value is equal to the expected value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfEq(path string, value, expected any) *Condition {
	r.condition = &Condition{
		satisfied:   value == expected,
		description: fmt.Sprintf("%s is equal to %v", path, expected),
	}
	return r.condition
}

// Condition also met if the value is equal to the expected value.
func (c *Condition) OrEq(path string, value, expected any) *Condition {
	c.description += fmt.Sprintf(" or %s is equal to %v", path, expected)
	c.satisfied = c.satisfied || value == expected
	return c
}

// Rule also satisfied if the value is equal to the expected value.
func (r *Rule) OrEq(path string, value, expected any) *Rule {
	r.description += fmt.Sprintf(" or %s must be equal to %v", path, expected)
	r.satisfied = r.satisfied || value == expected
	return r
}

// Require a primitive (string, number, bool) value at a given path to be greater than a given value.
//   - path: the path in the struct to check (e.g. "age" or "score")
//   - value: the value to compare against
//   - threshold: the threshold value
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) Gt(path string, value, threshold any) *Validator {
	v.GtR(path, value, threshold)
	return v
}

// Does the same as Gt, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) GtR(path string, value, threshold any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must be greater than %v", path, threshold),
		satisfied:   convertToFloat64(value) > convertToFloat64(threshold),
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value is greater than the threshold value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfGt(path string, value, threshold any) *Condition {
	r.condition = &Condition{
		satisfied:   convertToFloat64(value) > convertToFloat64(threshold),
		description: fmt.Sprintf("%s is greater than %v", path, threshold),
	}
	return r.condition
}

// Condition also met if the value is greater than the threshold value.
func (c *Condition) OrGt(path string, value, threshold any) *Condition {
	c.description += fmt.Sprintf(" or %s is greater than %v", path, threshold)
	c.satisfied = c.satisfied || convertToFloat64(value) > convertToFloat64(threshold)
	return c
}

// Rule also satisfied if the value is greater than the threshold value.
func (r *Rule) OrGt(path string, value, threshold any) *Rule {
	r.description += fmt.Sprintf(" or %s must be greater than %v", path, threshold)
	r.satisfied = r.satisfied || convertToFloat64(value) > convertToFloat64(threshold)
	return r
}

// Require a primitive (string, number, bool) value at a given path to be greater than or equal to a given value.
//   - path: the path in the struct to check (e.g. "age" or "score")
//   - value: the value to compare against
//   - threshold: the threshold value
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) Gte(path string, value, threshold any) *Validator {
	v.GteR(path, value, threshold)
	return v
}

// Does the same as Gte, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) GteR(path string, value, threshold any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must be greater than or equal to %v", path, threshold),
		satisfied:   convertToFloat64(value) >= convertToFloat64(threshold),
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value is greater than or equal to the threshold value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfGte(path string, value, threshold any) *Condition {
	r.condition = &Condition{
		satisfied:   convertToFloat64(value) >= convertToFloat64(threshold),
		description: fmt.Sprintf("%s is greater than or equal to %v", path, threshold),
	}
	return r.condition
}

// Condition also met if the value is greater than or equal to the threshold value.
func (c *Condition) OrGte(path string, value, threshold any) *Condition {
	c.description += fmt.Sprintf(" or %s is greater than or equal to %v", path, threshold)
	c.satisfied = c.satisfied || convertToFloat64(value) >= convertToFloat64(threshold)
	return c
}

// Rule also satisfied if the value is greater than or equal to the threshold value.
func (r *Rule) OrGte(path string, value, threshold any) *Rule {
	r.description += fmt.Sprintf(" or %s must be greater than or equal to %v", path, threshold)
	r.satisfied = r.satisfied || convertToFloat64(value) >= convertToFloat64(threshold)
	return r
}

// Require a primitive (string, number, bool) value at a given path to be less than a given value.
//   - path: the path in the struct to check (e.g. "age" or "score")
//   - value: the value to compare against
//   - threshold: the threshold value
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) Lt(path string, value, threshold any) *Validator {
	v.LtR(path, value, threshold)
	return v
}

// Does the same as Lt, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LtR(path string, value, threshold any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must be less than %v", path, threshold),
		satisfied:   convertToFloat64(value) < convertToFloat64(threshold),
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value is less than the threshold value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLt(path string, value, threshold any) *Condition {
	r.condition = &Condition{
		satisfied:   convertToFloat64(value) < convertToFloat64(threshold),
		description: fmt.Sprintf("%s is less than %v", path, threshold),
	}
	return r.condition
}

// Condition also met if the value is less than the threshold value.
func (c *Condition) OrLt(path string, value, threshold any) *Condition {
	c.description += fmt.Sprintf(" or %s is less than %v", path, threshold)
	c.satisfied = c.satisfied || convertToFloat64(value) < convertToFloat64(threshold)
	return c
}

// Rule also satisfied if the value is less than the threshold value.
func (r *Rule) OrLt(path string, value, threshold any) *Rule {
	r.description += fmt.Sprintf(" or %s must be less than %v", path, threshold)
	r.satisfied = r.satisfied || convertToFloat64(value) < convertToFloat64(threshold)
	return r
}

// Require a primitive (string, number, bool) value at a given path to be less than or equal to a given value.
//   - path: the path in the struct to check (e.g. "age" or "score")
//   - value: the value to compare against
//   - threshold: the threshold value
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) Lte(path string, value, threshold any) *Validator {
	v.LteR(path, value, threshold)
	return v
}

// Does the same as Lte, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LteR(path string, value, threshold any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must be less than or equal to %v", path, threshold),
		satisfied:   convertToFloat64(value) <= convertToFloat64(threshold),
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value is less than or equal to the threshold value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLte(path string, value, threshold any) *Condition {
	r.condition = &Condition{
		satisfied:   convertToFloat64(value) <= convertToFloat64(threshold),
		description: fmt.Sprintf("%s is less than or equal to %v", path, threshold),
	}
	return r.condition
}

// Condition also met if the value is less than or equal to the threshold value.
func (c *Condition) OrLte(path string, value, threshold any) *Condition {
	c.description += fmt.Sprintf(" or %s is less than or equal to %v", path, threshold)
	c.satisfied = c.satisfied || convertToFloat64(value) <= convertToFloat64(threshold)
	return c
}

// Rule also satisfied if the value is less than or equal to the threshold value.
func (r *Rule) OrLte(path string, value, threshold any) *Rule {
	r.description += fmt.Sprintf(" or %s must be less than or equal to %v", path, threshold)
	r.satisfied = r.satisfied || convertToFloat64(value) <= convertToFloat64(threshold)
	return r
}

// Require a primitive (string, number, bool) value at a given path to be one of the given values.
//   - path: the path in the struct to check (e.g. "status" or "type")
//   - value: the value to compare against
//   - options: the list of acceptable values
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) OneOf(path string, value any, options ...any) *Validator {
	v.OneOfR(path, value, options...)
	return v
}

// Does the same as OneOf, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) OneOfR(path string, value any, options ...any) *Rule {
	satisfied := false
	for _, option := range options {
		if value == option {
			satisfied = true
			break
		}
	}
	r := &Rule{
		description: fmt.Sprintf("%s must be one of %v", path, options),
		satisfied:   satisfied,
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value is one of the given options.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfOneOf(path string, value any, options ...any) *Condition {
	satisfied := false
	for _, option := range options {
		if value == option {
			satisfied = true
			break
		}
	}
	r.condition = &Condition{
		satisfied:   satisfied,
		description: fmt.Sprintf("%s is one of %v", path, options),
	}
	return r.condition
}

// Condition also met if the value is one of the given options.
func (c *Condition) OrOneOf(path string, value any, options ...any) *Condition {
	satisfied := false
	for _, option := range options {
		if value == option {
			satisfied = true
			break
		}
	}
	c.description += fmt.Sprintf(" or %s is one of %v", path, options)
	c.satisfied = c.satisfied || satisfied
	return c
}

// Rule also satisfied if the value is one of the given options.
func (r *Rule) OrOneOf(path string, value any, options ...any) *Rule {
	satisfied := false
	for _, option := range options {
		if value == option {
			satisfied = true
			break
		}
	}
	r.description += fmt.Sprintf(" or %s must be one of %v", path, options)
	r.satisfied = r.satisfied || satisfied
	return r
}

// Require a primitive (string, number, bool) value at a given path to not be equal to a given value.
//   - path: the path in the struct to check (e.g. "name" or "display_name")
//   - value: the value to compare against
//   - expected: the expected value
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) NotEq(path string, value, expected any) *Validator {
	v.NotEqR(path, value, expected)
	return v
}

// Does the same as NotEq, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) NotEqR(path string, value, actual any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must not be equal to %v", path, value),
		satisfied:   value != actual,
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value is not equal to the expected value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfNotEq(path string, value, expected any) *Condition {
	r.condition = &Condition{
		satisfied:   value != expected,
		description: fmt.Sprintf("%s is not equal to %v", path, expected),
	}
	return r.condition
}

// Condition also met if the value is not equal to the expected value.
func (c *Condition) OrNotEq(path string, value, expected any) *Condition {
	c.description += fmt.Sprintf(" or %s is not equal to %v", path, expected)
	c.satisfied = c.satisfied || value != expected
	return c
}

// Rule also satisfied if the value is not equal to the expected value.
func (r *Rule) OrNotEq(path string, value, expected any) *Rule {
	r.description += fmt.Sprintf(" or %s must not be equal to %v", path, expected)
	r.satisfied = r.satisfied || value != expected
	return r
}

// Require a primitive (string, number, bool) value at a given path to be none of the given values.
//   - path: the path in the struct to check (e.g. "status" or "type")
//   - value: the value to compare against
//   - options: the list of unacceptable values
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) NoneOf(path string, value any, options ...any) *Validator {
	v.NoneOfR(path, value, options...)
	return v
}

// Does the same as NoneOf, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) NoneOfR(path string, value any, options ...any) *Rule {
	satisfied := true
	for _, option := range options {
		if value == option {
			satisfied = false
			break
		}
	}
	r := &Rule{
		description: fmt.Sprintf("%s must be none of %v", path, options),
		satisfied:   satisfied,
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value is none of the given options.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfNoneOf(path string, value any, options ...any) *Condition {
	satisfied := true
	for _, option := range options {
		if value == option {
			satisfied = false
			break
		}
	}
	r.condition = &Condition{
		satisfied:   satisfied,
		description: fmt.Sprintf("%s is none of %v", path, options),
	}
	return r.condition
}

// Condition also met if the value is none of the given options.
func (c *Condition) OrNoneOf(path string, value any, options ...any) *Condition {
	satisfied := true
	for _, option := range options {
		if value == option {
			satisfied = false
			break
		}
	}
	c.description += fmt.Sprintf(" or %s is none of %v", path, options)
	c.satisfied = c.satisfied || satisfied
	return c
}

// Rule also satisfied if the value is none of the given options.
func (r *Rule) OrNoneOf(path string, value any, options ...any) *Rule {
	satisfied := true
	for _, option := range options {
		if value == option {
			satisfied = false
			break
		}
	}
	r.description += fmt.Sprintf(" or %s must be none of %v", path, options)
	r.satisfied = r.satisfied || satisfied
	return r
}

// Helper function to convert any type to float64
func convertToFloat64(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	default:
		return 0
	}
}
