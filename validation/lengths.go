package validation

// Require the length of a slice to be equal to a specific value.
func (v *Validator) LengthEq(field string, length int, expected int) *Validator {
	return v.Eq("length of "+field, length, expected)
}

// Does the same as LengthEq, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthEqR(path string, length int, expected int) *Rule {
	return v.EqR("length of "+path, length, expected)
}

// Only applies the rule if the length of the slice is equal to the expected value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthEq(path string, length int, expected int) *Condition {
	return r.IfEq("length of "+path, length, expected)
}

// Condition also met if the length of the slice is equal to the expected value.
func (c *Condition) OrLengthEq(path string, length int, expected int) *Condition {
	return c.OrEq("length of "+path, length, expected)
}

// Rule also satisfied if the value is equal to the expected value.
func (r *Rule) OrLengthEq(path string, length int, expected int) *Rule {
	return r.OrEq("length of "+path, length, expected)
}

// Require the length of a slice to not be equal to a specific value.
func (v *Validator) LengthNotEq(field string, length int, expected int) *Validator {
	return v.NotEq("length of "+field, length, expected)
}

// Does the same as LengthNotEq, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthNotEqR(path string, length int, expected int) *Rule {
	return v.NotEqR("length of "+path, length, expected)
}

// Only applies the rule if the length of the slice is not equal to the expected value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthNotEq(path string, length int, expected int) *Condition {
	return r.IfNotEq("length of "+path, length, expected)
}

// Condition also met if the length of the slice is not equal to the expected value.
func (c *Condition) OrLengthNotEq(path string, length int, expected int) *Condition {
	return c.OrNotEq("length of "+path, length, expected)
}

// Rule also satisfied if the value is not equal to the expected value.
func (r *Rule) OrLengthNotEq(path string, length int, expected int) *Rule {
	return r.OrNotEq("length of "+path, length, expected)
}

// Require the length of a slice to be greater than a specific value.
func (v *Validator) LengthGt(field string, length int, expected int) *Validator {
	return v.Gt("length of "+field, length, expected)
}

// Does the same as LengthGt, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthGtR(path string, length int, expected int) *Rule {
	return v.GtR("length of "+path, length, expected)
}

// Only applies the rule if the length of the slice is greater than the expected value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthGt(path string, length int, expected int) *Condition {
	return r.IfGt("length of "+path, length, expected)
}

// Condition also met if the length of the slice is greater than the expected value.
func (c *Condition) OrLengthGt(path string, length int, expected int) *Condition {
	return c.OrGt("length of "+path, length, expected)
}

// Rule also satisfied if the length is greater than the expected value.
func (r *Rule) OrLengthGt(path string, length int, expected int) *Rule {
	return r.OrGt("length of "+path, length, expected)
}

// Require the length of a slice to be greater than or equal to a specific value.
func (v *Validator) LengthGte(field string, length int, expected int) *Validator {
	return v.Gte("length of "+field, length, expected)
}

// Does the same as LengthGte, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthGteR(path string, length int, expected int) *Rule {
	return v.GteR("length of "+path, length, expected)
}

// Only applies the rule if the length of the slice is greater than or equal to the expected value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthGte(path string, length int, expected int) *Condition {
	return r.IfGte("length of "+path, length, expected)
}

// Condition also met if the length of the slice is greater than or equal to the expected value.
func (c *Condition) OrLengthGte(path string, length int, expected int) *Condition {
	return c.OrGte("length of "+path, length, expected)
}

// Rule also satisfied if the length is greater than or equal to the expected value.
func (r *Rule) OrLengthGte(path string, length int, expected int) *Rule {
	return r.OrGte("length of "+path, length, expected)
}

// Require the length of a slice to be less than a specific value.
func (v *Validator) LengthLt(field string, length int, expected int) *Validator {
	return v.Lt("length of "+field, length, expected)
}

// Does the same as LengthLt, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthLtR(path string, length int, expected int) *Rule {
	return v.LtR("length of "+path, length, expected)
}

// Only applies the rule if the length of the slice is less than the expected value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthLt(path string, length int, expected int) *Condition {
	return r.IfLt("length of "+path, length, expected)
}

// Condition also met if the length of the slice is less than the expected value.
func (c *Condition) OrLengthLt(path string, length int, expected int) *Condition {
	return c.OrLt("length of "+path, length, expected)
}

// Rule also satisfied if the length is less than the expected value.
func (r *Rule) OrLengthLt(path string, length int, expected int) *Rule {
	return r.OrLt("length of "+path, length, expected)
}

// Require the length of a slice to be less than or equal to a specific value.
func (v *Validator) LengthLte(field string, length int, expected int) *Validator {
	return v.Lte("length of "+field, length, expected)
}

// Does the same as LengthLte, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) LengthLteR(path string, length int, expected int) *Rule {
	return v.LteR("length of "+path, length, expected)
}

// Only applies the rule if the length of the slice is less than or equal to the expected value.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfLengthLte(path string, length int, expected int) *Condition {
	return r.IfLte("length of "+path, length, expected)
}

// Condition also met if the length of the slice is less than or equal to the expected value.
func (c *Condition) OrLengthLte(path string, length int, expected int) *Condition {
	return c.OrLte("length of "+path, length, expected)
}

// Rule also satisfied if the length is less than or equal to the expected value.
func (r *Rule) OrLengthLte(path string, length int, expected int) *Rule {
	return r.OrLte("length of "+path, length, expected)
}
