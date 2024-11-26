package validation

import (
	"regexp"
	"strings"
)

const (
	domainPattern     = `^([a-zA-Z0-9]+(-[a-zA-Z0-9]+)*\.)+[a-zA-Z]{2,}$`
	rootDomainPattern = `^([a-zA-Z0-9]+(-[a-zA-Z0-9]+)*\.)[a-zA-Z]{2,}$`
	subDomainPattern  = `^([a-zA-Z0-9]+(-[a-zA-Z0-9]+)*\.){2,}[a-zA-Z]{2,}$`
	emailPattern      = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
)

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

// Rule also satisfied if the string value is empty.
func (r *Rule) OrEmpty(path string, value string) *Rule {
	r.description += " or " + path + " must be empty"
	r.satisfied = r.satisfied || value == ""
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

// Require a string at a given path to start with a given prefix.
func (v *Validator) StartsWith(path string, value string, prefix string) *Validator {
	v.StartsWithR(path, value, prefix)
	return v
}

// Does the same as StartsWith, but returns the rule instead of the validator.
func (v *Validator) StartsWithR(path string, value string, prefix string) *Rule {
	r := &Rule{
		description: path + " must start with " + prefix,
		satisfied:   strings.HasPrefix(value, prefix),
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value starts with the given prefix.
func (r *Rule) OrStartsWith(path string, value string, prefix string) *Rule {
	r.description += " or " + path + " must start with " + prefix
	r.satisfied = r.satisfied || strings.HasPrefix(value, prefix)
	return r
}

// Only applies the rule if the string value starts with the given prefix.
func (r *Rule) IfStartsWith(path string, value string, prefix string) *Condition {
	r.condition = &Condition{
		satisfied:   strings.HasPrefix(value, prefix),
		description: path + " starts with " + prefix,
	}
	return r.condition
}

// Condition also met if the string value starts with the given prefix.
func (c *Condition) OrStartsWith(path string, value string, prefix string) *Condition {
	c.description += " or " + path + " starts with " + prefix
	c.satisfied = c.satisfied || strings.HasPrefix(value, prefix)
	return c
}

// Require a string at a given path to not start with a given prefix.
func (v *Validator) NotStartsWith(path string, value string, prefix string) *Validator {
	v.NotStartsWithR(path, value, prefix)
	return v
}

// Does the same as NotStartsWith, but returns the rule instead of the validator.
func (v *Validator) NotStartsWithR(path string, value string, prefix string) *Rule {
	r := &Rule{
		description: path + " must not start with " + prefix,
		satisfied:   !strings.HasPrefix(value, prefix),
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value does not start with the given prefix.
func (r *Rule) OrNotStartsWith(path string, value string, prefix string) *Rule {
	r.description += " or " + path + " must not start with " + prefix
	r.satisfied = r.satisfied || !strings.HasPrefix(value, prefix)
	return r
}

// Only applies the rule if the string value does not start with the given prefix.
func (r *Rule) IfNotStartsWith(path string, value string, prefix string) *Condition {
	r.condition = &Condition{
		satisfied:   !strings.HasPrefix(value, prefix),
		description: path + " does not start with " + prefix,
	}
	return r.condition
}

// Condition also met if the string value does not start with the given prefix.
func (c *Condition) OrNotStartsWith(path string, value string, prefix string) *Condition {
	c.description += " or " + path + " does not start with " + prefix
	c.satisfied = c.satisfied || !strings.HasPrefix(value, prefix)
	return c
}

// Require a string at a given path to end with a given suffix.
func (v *Validator) EndsWith(path string, value string, suffix string) *Validator {
	v.EndsWithR(path, value, suffix)
	return v
}

// Does the same as EndsWith, but returns the rule instead of the validator.
func (v *Validator) EndsWithR(path string, value string, suffix string) *Rule {
	r := &Rule{
		description: path + " must end with " + suffix,
		satisfied:   strings.HasSuffix(value, suffix),
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value ends with the given suffix.
func (r *Rule) OrEndsWith(path string, value string, suffix string) *Rule {
	r.description += " or " + path + " must end with " + suffix
	r.satisfied = r.satisfied || strings.HasSuffix(value, suffix)
	return r
}

// Only applies the rule if the string value ends with the given suffix.
func (r *Rule) IfEndsWith(path string, value string, suffix string) *Condition {
	r.condition = &Condition{
		satisfied:   strings.HasSuffix(value, suffix),
		description: path + " ends with " + suffix,
	}
	return r.condition
}

// Condition also met if the string value ends with the given suffix.
func (c *Condition) OrEndsWith(path string, value string, suffix string) *Condition {
	c.description += " or " + path + " ends with " + suffix
	c.satisfied = c.satisfied || strings.HasSuffix(value, suffix)
	return c
}

// Require a string at a given path to not end with a given suffix.
func (v *Validator) NotEndsWith(path string, value string, suffix string) *Validator {
	v.NotEndsWithR(path, value, suffix)
	return v
}

// Does the same as NotEndsWith, but returns the rule instead of the validator.
func (v *Validator) NotEndsWithR(path string, value string, suffix string) *Rule {
	r := &Rule{
		description: path + " must not end with " + suffix,
		satisfied:   !strings.HasSuffix(value, suffix),
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value does not end with the given suffix.
func (r *Rule) OrNotEndsWith(path string, value string, suffix string) *Rule {
	r.description += " or " + path + " must not end with " + suffix
	r.satisfied = r.satisfied || !strings.HasSuffix(value, suffix)
	return r
}

// Only applies the rule if the string value does not end with the given suffix.
func (r *Rule) IfNotEndsWith(path string, value string, suffix string) *Condition {
	r.condition = &Condition{
		satisfied:   !strings.HasSuffix(value, suffix),
		description: path + " does not end with " + suffix,
	}
	return r.condition
}

// Condition also met if the string value does not end with the given suffix.
func (c *Condition) OrNotEndsWith(path string, value string, suffix string) *Condition {
	c.description += " or " + path + " does not end with " + suffix
	c.satisfied = c.satisfied || !strings.HasSuffix(value, suffix)
	return c
}

// Require a string at a given path to contain a given substring.
func (v *Validator) Contains(path string, value string, substring string) *Validator {
	v.ContainsR(path, value, substring)
	return v
}

// Does the same as Contains, but returns the rule instead of the validator.
func (v *Validator) ContainsR(path string, value string, substring string) *Rule {
	r := &Rule{
		description: path + " must contain " + substring,
		satisfied:   strings.Contains(value, substring),
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value contains the given substring.
func (r *Rule) OrContains(path string, value string, substring string) *Rule {
	r.description += " or " + path + " must contain " + substring
	r.satisfied = r.satisfied || strings.Contains(value, substring)
	return r
}

// Only applies the rule if the string value contains the given substring.
func (r *Rule) IfContains(path string, value string, substring string) *Condition {
	r.condition = &Condition{
		satisfied:   strings.Contains(value, substring),
		description: path + " contains " + substring,
	}
	return r.condition
}

// Condition also met if the string value contains the given substring.
func (c *Condition) OrContains(path string, value string, substring string) *Condition {
	c.description += " or " + path + " contains " + substring
	c.satisfied = c.satisfied || strings.Contains(value, substring)
	return c
}

// Require a string at a given path to not contain a given substring.
func (v *Validator) NotContains(path string, value string, substring string) *Validator {
	v.NotContainsR(path, value, substring)
	return v
}

// Does the same as NotContains, but returns the rule instead of the validator.
func (v *Validator) NotContainsR(path string, value string, substring string) *Rule {
	r := &Rule{
		description: path + " must not contain " + substring,
		satisfied:   !strings.Contains(value, substring),
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value does not contain the given substring.
func (r *Rule) OrNotContains(path string, value string, substring string) *Rule {
	r.description += " or " + path + " must not contain " + substring
	r.satisfied = r.satisfied || !strings.Contains(value, substring)
	return r
}

// Only applies the rule if the string value does not contain the given substring.
func (r *Rule) IfNotContains(path string, value string, substring string) *Condition {
	r.condition = &Condition{
		satisfied:   !strings.Contains(value, substring),
		description: path + " does not contain " + substring,
	}
	return r.condition
}

// Condition also met if the string value does not contain the given substring.
func (c *Condition) OrNotContains(path string, value string, substring string) *Condition {
	c.description += " or " + path + " does not contain " + substring
	c.satisfied = c.satisfied || !strings.Contains(value, substring)
	return c
}

// Require a string at a given path to match a given regex pattern.
func (v *Validator) Matches(path string, value string, pattern string) *Validator {
	v.MatchesR(path, value, pattern)
	return v
}

// Does the same as Matches, but returns the rule instead of the validator.
func (v *Validator) MatchesR(path string, value string, pattern string) *Rule {
	matched, err := regexp.MatchString(pattern, value)
	r := &Rule{
		description: path + " must match pattern " + pattern,
		satisfied:   err == nil && matched,
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value matches the given regex pattern.
func (r *Rule) OrMatches(path string, value string, pattern string) *Rule {
	matched, err := regexp.MatchString(pattern, value)
	r.description += " or " + path + " must match pattern " + pattern
	r.satisfied = r.satisfied || (err == nil && matched)
	return r
}

// Only applies the rule if the string value matches the given regex pattern.
func (r *Rule) IfMatches(path string, value string, pattern string) *Condition {
	matched, err := regexp.MatchString(pattern, value)
	r.condition = &Condition{
		satisfied:   err == nil && matched,
		description: path + " matches pattern " + pattern,
	}
	return r.condition
}

// Condition also met if the string value matches the given regex pattern.
func (c *Condition) OrMatches(path string, value string, pattern string) *Condition {
	matched, err := regexp.MatchString(pattern, value)
	c.description += " or " + path + " matches pattern " + pattern
	c.satisfied = c.satisfied || (err == nil && matched)
	return c
}

// Require a string at a given path to match one of the given regex patterns.
func (v *Validator) MatchesOneOf(path string, value string, patterns ...string) *Validator {
	v.MatchesOneOfR(path, value, patterns...)
	return v
}

// Does the same as MatchesOneOf, but returns the rule instead of the validator.
func (v *Validator) MatchesOneOfR(path string, value string, patterns ...string) *Rule {
	satisfied := false
	for _, pattern := range patterns {
		matched, err := regexp.MatchString(pattern, value)
		if err == nil && matched {
			satisfied = true
			break
		}
	}
	r := &Rule{
		description: path + " must match one of the patterns: " + strings.Join(patterns, ", "),
		satisfied:   satisfied,
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value matches one of the given regex patterns.
func (r *Rule) OrMatchesOneOf(path string, value string, patterns ...string) *Rule {
	satisfied := false
	for _, pattern := range patterns {
		matched, err := regexp.MatchString(pattern, value)
		if err == nil && matched {
			satisfied = true
			break
		}
	}
	r.description += " or " + path + " must match one of the patterns: " + strings.Join(patterns, ", ")
	r.satisfied = r.satisfied || satisfied
	return r
}

// Only applies the rule if the string value matches one of the given regex patterns.
func (r *Rule) IfMatchesOneOf(path string, value string, patterns ...string) *Condition {
	satisfied := false
	for _, pattern := range patterns {
		matched, err := regexp.MatchString(pattern, value)
		if err == nil && matched {
			satisfied = true
			break
		}
	}
	r.condition = &Condition{
		satisfied:   satisfied,
		description: path + " matches one of the patterns: " + strings.Join(patterns, ", "),
	}
	return r.condition
}

// Condition also met if the string value matches one of the given regex patterns.
func (c *Condition) OrMatchesOneOf(path string, value string, patterns ...string) *Condition {
	satisfied := false
	for _, pattern := range patterns {
		matched, err := regexp.MatchString(pattern, value)
		if err == nil && matched {
			satisfied = true
			break
		}
	}
	c.description += " or " + path + " matches one of the patterns: " + strings.Join(patterns, ", ")
	c.satisfied = c.satisfied || satisfied
	return c
}

// Require a string at a given path to be a valid domain.
func (v *Validator) Domain(path string, value string) *Validator {
	v.DomainR(path, value)
	return v
}

// Does the same as Domain, but returns the rule instead of the validator.
func (v *Validator) DomainR(path string, value string) *Rule {
	matched, err := regexp.MatchString(domainPattern, value)
	r := &Rule{
		description: path + " must be a valid domain",
		satisfied:   err == nil && matched,
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value is a valid domain.
func (r *Rule) OrDomain(path string, value string) *Rule {
	matched, err := regexp.MatchString(domainPattern, value)
	r.description += " or " + path + " must be a valid domain"
	r.satisfied = r.satisfied || (err == nil && matched)
	return r
}

// Only applies the rule if the string value is a valid domain.
func (r *Rule) IfDomain(path string, value string) *Condition {
	matched, err := regexp.MatchString(domainPattern, value)
	r.condition = &Condition{
		satisfied:   err == nil && matched,
		description: path + " is a valid domain",
	}
	return r.condition
}

// Condition also met if the string value is a valid domain.
func (c *Condition) OrDomain(path string, value string) *Condition {
	matched, err := regexp.MatchString(domainPattern, value)
	c.description += " or " + path + " is a valid domain"
	c.satisfied = c.satisfied || (err == nil && matched)
	return c
}

// Require a string at a given path to be a valid root domain.
func (v *Validator) RootDomain(path string, value string) *Validator {
	v.RootDomainR(path, value)
	return v
}

// Does the same as RootDomain, but returns the rule instead of the validator.
func (v *Validator) RootDomainR(path string, value string) *Rule {
	matched, err := regexp.MatchString(rootDomainPattern, value)
	r := &Rule{
		description: path + " must be a valid root domain",
		satisfied:   err == nil && matched,
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value is a valid root domain.
func (r *Rule) OrRootDomain(path string, value string) *Rule {
	matched, err := regexp.MatchString(rootDomainPattern, value)
	r.description += " or " + path + " must be a valid root domain"
	r.satisfied = r.satisfied || (err == nil && matched)
	return r
}

// Only applies the rule if the string value is a valid root domain.
func (r *Rule) IfRootDomain(path string, value string) *Condition {
	matched, err := regexp.MatchString(rootDomainPattern, value)
	r.condition = &Condition{
		satisfied:   err == nil && matched,
		description: path + " is a valid root domain",
	}
	return r.condition
}

// Condition also met if the string value is a valid root domain.
func (c *Condition) OrRootDomain(path string, value string) *Condition {
	matched, err := regexp.MatchString(rootDomainPattern, value)
	c.description += " or " + path + " is a valid root domain"
	c.satisfied = c.satisfied || (err == nil && matched)
	return c
}

// Require a string at a given path to be a valid subdomain.
func (v *Validator) SubDomain(path string, value string) *Validator {
	v.SubDomainR(path, value)
	return v
}

// Does the same as SubDomain, but returns the rule instead of the validator.
func (v *Validator) SubDomainR(path string, value string) *Rule {
	matched, err := regexp.MatchString(subDomainPattern, value)
	r := &Rule{
		description: path + " must be a valid subdomain",
		satisfied:   err == nil && matched,
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value is a valid subdomain.
func (r *Rule) OrSubDomain(path string, value string) *Rule {
	matched, err := regexp.MatchString(subDomainPattern, value)
	r.description += " or " + path + " must be a valid subdomain"
	r.satisfied = r.satisfied || (err == nil && matched)
	return r
}

// Only applies the rule if the string value is a valid subdomain.
func (r *Rule) IfSubDomain(path string, value string) *Condition {
	matched, err := regexp.MatchString(subDomainPattern, value)
	r.condition = &Condition{
		satisfied:   err == nil && matched,
		description: path + " is a valid subdomain",
	}
	return r.condition
}

// Condition also met if the string value is a valid subdomain.
func (c *Condition) OrSubDomain(path string, value string) *Condition {
	matched, err := regexp.MatchString(subDomainPattern, value)
	c.description += " or " + path + " is a valid subdomain"
	c.satisfied = c.satisfied || (err == nil && matched)
	return c
}

// Require a string at a given path to be a valid email.
func (v *Validator) Email(path string, value string) *Validator {
	v.EmailR(path, value)
	return v
}

// Does the same as Email, but returns the rule instead of the validator.
func (v *Validator) EmailR(path string, value string) *Rule {
	matched, err := regexp.MatchString(emailPattern, value)
	r := &Rule{
		description: path + " must be a valid email",
		satisfied:   err == nil && matched,
	}
	v.rules = append(v.rules, r)
	return r
}

// Rule also satisfied if the string value is a valid email.
func (r *Rule) OrEmail(path string, value string) *Rule {
	matched, err := regexp.MatchString(emailPattern, value)
	r.description += " or " + path + " must be a valid email"
	r.satisfied = r.satisfied || (err == nil && matched)
	return r
}

// Only applies the rule if the string value is a valid email.
func (r *Rule) IfEmail(path string, value string) *Condition {
	matched, err := regexp.MatchString(emailPattern, value)
	r.condition = &Condition{
		satisfied:   err == nil && matched,
		description: path + " is a valid email",
	}
	return r.condition
}

// Condition also met if the string value is a valid email.
func (c *Condition) OrEmail(path string, value string) *Condition {
	matched, err := regexp.MatchString(emailPattern, value)
	c.description += " or " + path + " is a valid email"
	c.satisfied = c.satisfied || (err == nil && matched)
	return c
}
