package validation

// Require a string at a given path to be populated.
//   - path: the path in the struct to check (e.g. "name" or "display_name")
//   - value: the value of the string
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) Populated(path string, value string) *Validator {
	v.PopulatedR(path, value)
	return v
}

// Does the same as Populated, but returns the added rule instead of the validator.
// Useful for adding dependencies (e.g. r3.DependsOn(r1,r2)) or conditions (e.g. r3.IfEq(c1,c2))
func (v *Validator) PopulatedR(path string, value string) *Rule {
	r := &Rule{
		description: path + " must be populated",
		satisfied:   value != "",
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value is populated.
func (r *Rule) OrPopulated(path string, value string) *Rule {
	r.description += " or " + path + " must populated"
	r.satisfied = r.satisfied || value != ""
	return r
}

// Only applies the rule if the stirng value is populated.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfPopulated(path string, value string) *Condition {
	r.condition = &Condition{
		satisfied:   value != "",
		description: path + " is populated",
	}
	return r.condition
}

// Condition also met if the string value is populated.
func (c *Condition) OrPopulated(path string, value string) *Condition {
	c.description += " or " + path + " is populated"
	c.satisfied = c.satisfied || value != ""
	return c
}

// Require a string at a given path to empty.
//   - path: the path in the struct to check (e.g. "name" or "display_name")
//   - value: the value of the string
//
// Returns the validator instance to allow for method chaining.
func (v *Validator) Empty(path string, value string) *Validator {
	v.EmptyR(path, value)
	return v
}

// Does the same as Empty, but returns the rule instead of the validator.
// Useful for adding rule dependencies e.g. r3.DependsOn(r1,r2)
func (v *Validator) EmptyR(path string, value string) *Rule {
	r := &Rule{
		description: path + " must be empty",
		satisfied:   value == "",
	}
	v.rules = append(v.rules, r)
	return r
}

// Only applies the rule if the value is empty.
// Returns new condition in order to "or" conditions together.
func (r *Rule) IfEmpty(path string, value string) *Condition {
	r.condition = &Condition{
		satisfied:   value == "",
		description: path + " is empty",
	}
	return r.condition
}

// Condition also met if the value is empty.
func (c *Condition) OrEmpty(path string, value string) *Condition {
	c.description += " or " + path + " is empty"
	c.satisfied = c.satisfied || value == ""
	return c
}
