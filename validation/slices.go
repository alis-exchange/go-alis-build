package validation

import "fmt"

// Require the length of a value at a given path to be equal to a given length.
//   - path: the path in the struct to check
//   - value: the value to check the length of
//   - expected: the expected length
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) LengthEq(path string, value []any, expected int) *Validator {
	v.LengthEqR(path, value, expected)
	return v
}

// Does the same as LengthEq, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthEqR(path string, value []any, expected int) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s length must be equal to %d", path, expected),
		satisfied:   len(value) == expected,
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the length of the value is equal to the expected length.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthEq(path string, value []any, expected int) *Condition {
	r.condition = &Condition{
		satisfied:   len(value) == expected,
		description: fmt.Sprintf("%s length is equal to %d", path, expected),
	}
	return r.condition
}

// Condition also met if the length of the value is equal to the expected length.
func (c *Condition) OrLengthEq(path string, value []any, expected int) *Condition {
	c.description += fmt.Sprintf(" or %s length is equal to %d", path, expected)
	c.satisfied = c.satisfied || len(value) == expected
	return c
}

// Rule also satisfied if the length of the value is equal to the expected length.
func (r *Rule) OrLengthEq(path string, value []any, expected int) *Rule {
	r.description += fmt.Sprintf(" or %s length must be equal to %d", path, expected)
	r.satisfied = r.satisfied || len(value) == expected
	return r
}

// Require the length of a value at a given path to not be equal to a given length.
//   - path: the path in the struct to check
//   - value: the value to check the length of
//   - expected: the length that should not match
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) LengthNotEq(path string, value []any, expected int) *Validator {
	v.LengthNotEqR(path, value, expected)
	return v
}

// Does the same as LengthNotEq, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthNotEqR(path string, value []any, expected int) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s length must not be equal to %d", path, expected),
		satisfied:   len(value) != expected,
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the length of the value is not equal to the expected length.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthNotEq(path string, value []any, expected int) *Condition {
	r.condition = &Condition{
		satisfied:   len(value) != expected,
		description: fmt.Sprintf("%s length is not equal to %d", path, expected),
	}
	return r.condition
}

// Condition also met if the length of the value is not equal to the expected length.
func (c *Condition) OrLengthNotEq(path string, value []any, expected int) *Condition {
	c.description += fmt.Sprintf(" or %s length is not equal to %d", path, expected)
	c.satisfied = c.satisfied || len(value) != expected
	return c
}

// Rule also satisfied if the length of the value is not equal to the expected length.
func (r *Rule) OrLengthNotEq(path string, value []any, expected int) *Rule {
	r.description += fmt.Sprintf(" or %s length must not be equal to %d", path, expected)
	r.satisfied = r.satisfied || len(value) != expected
	return r
}

// Require the length of a value at a given path to be greater than a given length.
//   - path: the path in the struct to check
//   - value: the value to check the length of
//   - threshold: the threshold length
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) LengthGt(path string, value []any, threshold int) *Validator {
	v.LengthGtR(path, value, threshold)
	return v
}

// Does the same as LengthGt, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthGtR(path string, value []any, threshold int) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s length must be greater than %d", path, threshold),
		satisfied:   len(value) > threshold,
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the length of the value is greater than the threshold length.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthGt(path string, value []any, threshold int) *Condition {
	r.condition = &Condition{
		satisfied:   len(value) > threshold,
		description: fmt.Sprintf("%s length is greater than %d", path, threshold),
	}
	return r.condition
}

// Condition also met if the length of the value is greater than the threshold length.
func (c *Condition) OrLengthGt(path string, value []any, threshold int) *Condition {
	c.description += fmt.Sprintf(" or %s length is greater than %d", path, threshold)
	c.satisfied = c.satisfied || len(value) > threshold
	return c
}

// Rule also satisfied if the length of the value is greater than the threshold length.
func (r *Rule) OrLengthGt(path string, value []any, threshold int) *Rule {
	r.description += fmt.Sprintf(" or %s length must be greater than %d", path, threshold)
	r.satisfied = r.satisfied || len(value) > threshold
	return r
}

// Require the length of a value at a given path to be greater than or equal to a given length.
//   - path: the path in the struct to check
//   - value: the value to check the length of
//   - threshold: the threshold length
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) LengthGte(path string, value []any, threshold int) *Validator {
	v.LengthGteR(path, value, threshold)
	return v
}

// Does the same as LengthGte, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthGteR(path string, value []any, threshold int) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s length must be greater than or equal to %d", path, threshold),
		satisfied:   len(value) >= threshold,
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the length of the value is greater than or equal to the threshold length.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthGte(path string, value []any, threshold int) *Condition {
	r.condition = &Condition{
		satisfied:   len(value) >= threshold,
		description: fmt.Sprintf("%s length is greater than or equal to %d", path, threshold),
	}
	return r.condition
}

// Condition also met if the length of the value is greater than or equal to the threshold length.
func (c *Condition) OrLengthGte(path string, value []any, threshold int) *Condition {
	c.description += fmt.Sprintf(" or %s length is greater than or equal to %d", path, threshold)
	c.satisfied = c.satisfied || len(value) >= threshold
	return c
}

// Rule also satisfied if the length of the value is greater than or equal to the threshold length.
func (r *Rule) OrLengthGte(path string, value []any, threshold int) *Rule {
	r.description += fmt.Sprintf(" or %s length must be greater than or equal to %d", path, threshold)
	r.satisfied = r.satisfied || len(value) >= threshold
	return r
}

// Require the length of a value at a given path to be less than a given length.
//   - path: the path in the struct to check
//   - value: the value to check the length of
//   - threshold: the threshold length
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) LengthLt(path string, value []any, threshold int) *Validator {
	v.LengthLtR(path, value, threshold)
	return v
}

// Does the same as LengthLt, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthLtR(path string, value []any, threshold int) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s length must be less than %d", path, threshold),
		satisfied:   len(value) < threshold,
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the length of the value is less than the threshold length.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthLt(path string, value []any, threshold int) *Condition {
	r.condition = &Condition{
		satisfied:   len(value) < threshold,
		description: fmt.Sprintf("%s length is less than %d", path, threshold),
	}
	return r.condition
}

// Condition also met if the length of the value is less than the threshold length.
func (c *Condition) OrLengthLt(path string, value []any, threshold int) *Condition {
	c.description += fmt.Sprintf(" or %s length is less than %d", path, threshold)
	c.satisfied = c.satisfied || len(value) < threshold
	return c
}

// Rule also satisfied if the length of the value is less than the threshold length.
func (r *Rule) OrLengthLt(path string, value []any, threshold int) *Rule {
	r.description += fmt.Sprintf(" or %s length must be less than %d", path, threshold)
	r.satisfied = r.satisfied || len(value) < threshold
	return r
}

// Require the length of a value at a given path to be less than or equal to a given length.
//   - path: the path in the struct to check
//   - value: the value to check the length of
//   - threshold: the threshold length
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) LengthLte(path string, value []any, threshold int) *Validator {
	v.LengthLteR(path, value, threshold)
	return v
}

// Does the same as LengthLte, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthLteR(path string, value []any, threshold int) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s length must be less than or equal to %d", path, threshold),
		satisfied:   len(value) <= threshold,
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the length of the value is less than or equal to the threshold length.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthLte(path string, value []any, threshold int) *Condition {
	r.condition = &Condition{
		satisfied:   len(value) <= threshold,
		description: fmt.Sprintf("%s length is less than or equal to %d", path, threshold),
	}
	return r.condition
}

// Condition also met if the length of the value is less than or equal to the threshold length.
func (c *Condition) OrLengthLte(path string, value []any, threshold int) *Condition {
	c.description += fmt.Sprintf(" or %s length is less than or equal to %d", path, threshold)
	c.satisfied = c.satisfied || len(value) <= threshold
	return c
}

// Rule also satisfied if the length of the value is less than or equal to the threshold length.
func (r *Rule) OrLengthLte(path string, value []any, threshold int) *Rule {
	r.description += fmt.Sprintf(" or %s length must be less than or equal to %d", path, threshold)
	r.satisfied = r.satisfied || len(value) <= threshold
	return r
}

// Require the value at a given path to include at least one of the specified elements.
//   - path: the path in the struct to check
//   - value: the value to check
//   - elements: the elements to check for inclusion
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) IncludesSome(path string, value []any, elements []any) *Validator {
	v.IncludesSomeR(path, value, elements)
	return v
}

// Does the same as IncludesSome, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) IncludesSomeR(path string, value []any, elements []any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must include at least one of %v", path, elements),
		satisfied:   includesSome(value, elements),
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value includes at least one of the specified elements.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfIncludesSome(path string, value []any, elements []any) *Condition {
	r.condition = &Condition{
		satisfied:   includesSome(value, elements),
		description: fmt.Sprintf("%s includes at least one of %v", path, elements),
	}
	return r.condition
}

// Condition also met if the value includes at least one of the specified elements.
func (c *Condition) OrIncludesSome(path string, value []any, elements []any) *Condition {
	c.description += fmt.Sprintf(" or %s includes at least one of %v", path, elements)
	c.satisfied = c.satisfied || includesSome(value, elements)
	return c
}

// Rule also satisfied if the value includes at least one of the specified elements.
func (r *Rule) OrIncludesSome(path string, value []any, elements []any) *Rule {
	r.description += fmt.Sprintf(" or %s must include at least one of %v", path, elements)
	r.satisfied = r.satisfied || includesSome(value, elements)
	return r
}

// Require the value at a given path to include all of the specified elements.
//   - path: the path in the struct to check
//   - value: the value to check
//   - elements: the elements to check for inclusion
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) IncludesAll(path string, value []any, elements []any) *Validator {
	v.IncludesAllR(path, value, elements)
	return v
}

// Does the same as IncludesAll, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) IncludesAllR(path string, value []any, elements []any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must include all of %v", path, elements),
		satisfied:   includesAll(value, elements),
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value includes all of the specified elements.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfIncludesAll(path string, value []any, elements []any) *Condition {
	r.condition = &Condition{
		satisfied:   includesAll(value, elements),
		description: fmt.Sprintf("%s includes all of %v", path, elements),
	}
	return r.condition
}

// Condition also met if the value includes all of the specified elements.
func (c *Condition) OrIncludesAll(path string, value []any, elements []any) *Condition {
	c.description += fmt.Sprintf(" or %s includes all of %v", path, elements)
	c.satisfied = c.satisfied || includesAll(value, elements)
	return c
}

// Rule also satisfied if the value includes all of the specified elements.
func (r *Rule) OrIncludesAll(path string, value []any, elements []any) *Rule {
	r.description += fmt.Sprintf(" or %s must include all of %v", path, elements)
	r.satisfied = r.satisfied || includesAll(value, elements)
	return r
}

// Require the value at a given path to only include the specified elements.
//   - path: the path in the struct to check
//   - value: the value to check
//   - elements: the elements to check for inclusion
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) OnlyIncludes(path string, value []any, elements []any) *Validator {
	v.OnlyIncludesR(path, value, elements)
	return v
}

// Does the same as OnlyIncludes, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) OnlyIncludesR(path string, value []any, elements []any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must only include %v", path, elements),
		satisfied:   onlyIncludes(value, elements),
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value only includes the specified elements.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfOnlyIncludes(path string, value []any, elements []any) *Condition {
	r.condition = &Condition{
		satisfied:   onlyIncludes(value, elements),
		description: fmt.Sprintf("%s only includes %v", path, elements),
	}
	return r.condition
}

// Condition also met if the value only includes the specified elements.
func (c *Condition) OrOnlyIncludes(path string, value []any, elements []any) *Condition {
	c.description += fmt.Sprintf(" or %s only includes %v", path, elements)
	c.satisfied = c.satisfied || onlyIncludes(value, elements)
	return c
}

// Rule also satisfied if the value only includes the specified elements.
func (r *Rule) OrOnlyIncludes(path string, value []any, elements []any) *Rule {
	r.description += fmt.Sprintf(" or %s must only include %v", path, elements)
	r.satisfied = r.satisfied || onlyIncludes(value, elements)
	return r
}

// Require the value at a given path to exclude all of the specified elements.
//   - path: the path in the struct to check
//   - value: the value to check
//   - elements: the elements to check for exclusion
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) Excludes(path string, value []any, elements []any) *Validator {
	v.ExcludesR(path, value, elements)
	return v
}

// Does the same as Excludes, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) ExcludesR(path string, value []any, elements []any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must exclude %v", path, elements),
		satisfied:   excludes(value, elements),
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value excludes all of the specified elements.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfExcludes(path string, value []any, elements []any) *Condition {
	r.condition = &Condition{
		satisfied:   excludes(value, elements),
		description: fmt.Sprintf("%s excludes %v", path, elements),
	}
	return r.condition
}

// Condition also met if the value excludes all of the specified elements.
func (c *Condition) OrExcludes(path string, value []any, elements []any) *Condition {
	c.description += fmt.Sprintf(" or %s excludes %v", path, elements)
	c.satisfied = c.satisfied || excludes(value, elements)
	return c
}

// Rule also satisfied if the value excludes all of the specified elements.
func (r *Rule) OrExcludes(path string, value []any, elements []any) *Rule {
	r.description += fmt.Sprintf(" or %s must exclude %v", path, elements)
	r.satisfied = r.satisfied || excludes(value, elements)
	return r
}

// Require all elements in the value at a given path to be unique.
//   - path: the path in the struct to check
//   - value: the value to check for uniqueness
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) Unique(path string, value []any) *Validator {
	v.UniqueR(path, value)
	return v
}

// Does the same as Unique, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) UniqueR(path string, value []any) *Rule {
	r := &Rule{
		description: fmt.Sprintf("%s must have unique elements", path),
		satisfied:   unique(value),
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if all elements in the value are unique.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfUnique(path string, value []any) *Condition {
	r.condition = &Condition{
		satisfied:   unique(value),
		description: fmt.Sprintf("%s has unique elements", path),
	}
	return r.condition
}

// Condition also met if all elements in the value are unique.
func (c *Condition) OrUnique(path string, value []any) *Condition {
	c.description += fmt.Sprintf(" or %s has unique elements", path)
	c.satisfied = c.satisfied || unique(value)
	return c
}

// Rule also satisfied if all elements in the value are unique.
func (r *Rule) OrUnique(path string, value []any) *Rule {
	r.description += fmt.Sprintf(" or %s must have unique elements", path)
	r.satisfied = r.satisfied || unique(value)
	return r
}

// Helper function to check uniqueness
func unique(value []any) bool {
	elementSet := make(map[any]struct{})
	for _, v := range value {
		if _, found := elementSet[v]; found {
			return false
		}
		elementSet[v] = struct{}{}
	}
	return true
}

// Helper functions to check inclusion/exclusion
func includesSome(value []any, elements []any) bool {
	for _, e := range elements {
		for _, v := range value {
			if v == e {
				return true
			}
		}
	}
	return false
}

func includesAll(value []any, elements []any) bool {
	for _, e := range elements {
		found := false
		for _, v := range value {
			if v == e {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func onlyIncludes(value []any, elements []any) bool {
	elementSet := make(map[any]struct{})
	for _, e := range elements {
		elementSet[e] = struct{}{}
	}
	for _, v := range value {
		if _, found := elementSet[v]; !found {
			return false
		}
	}
	return true
}

func excludes(value []any, elements []any) bool {
	for _, e := range elements {
		for _, v := range value {
			if v == e {
				return false
			}
		}
	}
	return true
}
