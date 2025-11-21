package validation

import (
	"regexp"
	"strings"
)

const (
	emailRgx      = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	domainRgx     = `^([a-zA-Z0-9]+(-[a-zA-Z0-9]+)*\.)+[a-zA-Z]{2,}$`
	rootDomainRgx = `^([a-zA-Z0-9]+(-[a-zA-Z0-9]+)*\.)[a-zA-Z]{2,}$`
	subDomainRgx  = `^([a-zA-Z0-9]+(-[a-zA-Z0-9]+)*\.){2,}[a-zA-Z]{2,}$`
	httpsUrlRgx   = `^https://(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`
)

// String provides validation rules for string values.
type String struct {
	standard[string]
}

// IsPopulated adds a rule asserting that the string must not be empty.
func (s *String) IsPopulated() *String {
	s.add("be populated", "is populated", s.value != "")
	return s
}

// IsEmpty adds a rule asserting that the string must be empty.
func (s *String) IsEmpty() *String {
	s.add("be empty", "is empty", s.value == "")
	return s
}

// Eq adds a rule asserting that the string must be equal to the given value.
func (s *String) Eq(eq string) *String {
	s.add("be equal to %v", "is equal to %v", s.value == eq, eq)
	return s
}

// NotEq adds a rule asserting that the string must not be equal to the given value.
func (s *String) NotEq(neq string) *String {
	s.add("not be equal to %v", "is not equal to %v", s.value != neq, neq)
	return s
}

// IsOneof adds a rule asserting that the string must be one of the given values.
func (s *String) IsOneof(values ...string) *String {
	satisfied := false
	for _, v := range values {
		if s.value == v {
			satisfied = true
			break
		}
	}
	s.add("be one of %v", "is one of %v", satisfied, values)
	return s
}

// IsNoneof adds a rule asserting that the string must not be any of the given values.
func (s *String) IsNoneof(values ...string) *String {
	satisfied := true
	for _, v := range values {
		if s.value == v {
			satisfied = false
			break
		}
	}
	s.add("be none of %v", "is none of %v", satisfied, values)
	return s
}

// StartsWith adds a rule asserting that the string must start with the given prefix.
func (s *String) StartsWith(prefix string) *String {
	s.add("start with '%v'", "starts with '%v'", strings.HasPrefix(s.value, prefix), prefix)
	return s
}

// StartsWithOneof adds a rule asserting that the string must start with one of the given prefixes.
func (s *String) StartsWithOneof(prefixes ...string) *String {
	satisfied := false
	for _, prefix := range prefixes {
		if strings.HasPrefix(s.value, prefix) {
			satisfied = true
			break
		}
	}
	s.add("start with one of %v", "starts with one of %v", satisfied, prefixes)
	return s
}

// HasNoneofPrefixes adds a rule asserting that the string must not start with any of the given prefixes.
func (s *String) HasNoneofPrefixes(prefixes ...string) *String {
	satisfied := true
	for _, prefix := range prefixes {
		if strings.HasPrefix(s.value, prefix) {
			satisfied = false
			break
		}
	}
	s.add("start with none of %v", "starts with none of %v", satisfied, prefixes)
	return s
}

// EndsWith adds a rule asserting that the string must end with the given suffix.
func (s *String) EndsWith(suffix string) *String {
	s.add("end with %v", "ends with %v", strings.HasSuffix(s.value, suffix), suffix)
	return s
}

// Contains adds a rule asserting that the string must contain the given substring.
func (s *String) Contains(substr string) *String {
	s.add("contain %v", "contains %v", strings.Contains(s.value, substr), substr)
	return s
}

// NotContains adds a rule asserting that the string must not contain the given substring.
func (s *String) NotContains(substr string) *String {
	s.add("not contain %v", "does not contain %v", !strings.Contains(s.value, substr), substr)
	return s
}

// LenEq adds a rule asserting that the string's length must be equal to the given length.
func (s *String) LenEq(length int) *String {
	s.add("have length equal to %v", "has length equal to %v", len(s.value) == length, length)
	return s
}

// LenGt adds a rule asserting that the string's length must be strictly greater than the given length.
func (s *String) LenGt(length int) *String {
	s.add("have length greater than %v", "has length greater than %v", len(s.value) > length, length)
	return s
}

// LenGte adds a rule asserting that the string's length must be greater than or equal to the given length.
func (s *String) LenGte(length int) *String {
	s.add("have length greater than or equal to %v", "has length greater than or equal to %v", len(s.value) >= length, length)
	return s
}

// LenLt adds a rule asserting that the string's length must be strictly less than the given length.
func (s *String) LenLt(length int) *String {
	s.add("have length less than %v", "has length less than %v", len(s.value) < length, length)
	return s
}

// LenLte adds a rule asserting that the string's length must be less than or equal to the given length.
func (s *String) LenLte(length int) *String {
	s.add("have length less than or equal to %v", "has length less than or equal to %v", len(s.value) <= length, length)
	return s
}

// Matches adds a rule asserting that the string must match the given regular expression pattern.
func (s *String) Matches(pattern string) *String {
	satisfied, err := regexp.MatchString(pattern, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("match %v", "matches %v", satisfied, pattern)
	return s
}

// MatchesOneof adds a rule asserting that the string must match at least one of the given regular expression patterns.
func (s *String) MatchesOneof(patterns ...string) *String {
	satisfied := false
	for _, pattern := range patterns {
		if matched, err := regexp.MatchString(pattern, s.value); err == nil && matched {
			satisfied = true
			break
		}
	}
	s.add("match one of %v", "matches one of %v", satisfied, patterns)
	return s
}

// MatchesNoneof adds a rule asserting that the string must not match any of the given regular expression patterns.
func (s *String) MatchesNoneof(patterns ...string) *String {
	satisfied := true
	for _, pattern := range patterns {
		if matched, err := regexp.MatchString(pattern, s.value); err == nil && matched {
			satisfied = false
			break
		}
	}
	s.add("match none of %v", "matches none of %v", satisfied, patterns)
	return s
}

// NotMatch adds a rule asserting that the string must not match the given regular expression pattern.
func (s *String) NotMatch(pattern string) *String {
	satisfied, err := regexp.MatchString(pattern, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("not match %v", "does not match %v", !satisfied, pattern)
	return s
}

// StringValidationOptions holds configuration options for string validation.
type StringValidationOptions struct {
	allowEmpty bool
}

// StringValidationOption is a function type for modifying StringValidationOptions.
type StringValidationOption func(*StringValidationOptions)

// AllowEmptyString is an option that allows empty strings to pass validation even if they don't match the specific format (e.g. email).
func AllowEmptyString() StringValidationOption {
	return func(options *StringValidationOptions) {
		options.allowEmpty = true
	}
}

// mergeStringValidationOptions applies the given options to a default StringValidationOptions instance.
func mergeStringValidationOptions(opts ...StringValidationOption) *StringValidationOptions {
	options := &StringValidationOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

// IsEmail adds a rule asserting that the string must be a valid email address.
// Use AllowEmptyString() option to consider empty strings as valid.
func (s *String) IsEmail(opts ...StringValidationOption) *String {
	options := mergeStringValidationOptions(opts...)
	satisfied, err := regexp.MatchString(emailRgx, s.value)
	if err != nil {
		satisfied = false
	}
	if options.allowEmpty {
		if s.value == "" {
			satisfied = true
		}
		s.add("be empty or a valid email", "is empty or a valid email", satisfied)
	} else {
		s.add("be a valid email", "is a valid email", satisfied)
	}
	return s
}

// IsDomain adds a rule asserting that the string must be a valid domain name.
// Use AllowEmptyString() option to consider empty strings as valid.
func (s *String) IsDomain(opts ...StringValidationOption) *String {
	options := mergeStringValidationOptions(opts...)
	satisfied, err := regexp.MatchString(domainRgx, s.value)
	if err != nil {
		satisfied = false
	}
	if options.allowEmpty {
		if s.value == "" {
			satisfied = true
		}
		s.add("be empty or a valid domain", "is empty or a valid domain", satisfied)
	} else {
		s.add("be a valid domain", "is a valid domain", satisfied)
	}
	return s
}

// IsRootDomain adds a rule asserting that the string must be a valid root domain (e.g., example.com, not sub.example.com).
// Use AllowEmptyString() option to consider empty strings as valid.
func (s *String) IsRootDomain(opts ...StringValidationOption) *String {
	options := mergeStringValidationOptions(opts...)
	satisfied, err := regexp.MatchString(rootDomainRgx, s.value)
	if err != nil {
		satisfied = false
	}
	if options.allowEmpty {
		if s.value == "" {
			satisfied = true
		}
		s.add("be empty or a valid root domain", "is empty or a valid root domain", satisfied)
	} else {
		s.add("be a valid root domain", "is a valid root domain", satisfied)
	}
	return s
}

// IsSubDomain adds a rule asserting that the string must be a valid subdomain (e.g., sub.example.com).
// Use AllowEmptyString() option to consider empty strings as valid.
func (s *String) IsSubDomain(opts ...StringValidationOption) *String {
	options := mergeStringValidationOptions(opts...)
	satisfied, err := regexp.MatchString(subDomainRgx, s.value)
	if err != nil {
		satisfied = false
	}
	if options.allowEmpty {
		if s.value == "" {
			satisfied = true
		}
		s.add("be empty or a valid sub domain", "is empty or a valid sub domain", satisfied)
	} else {
		s.add("be a valid sub domain", "is a valid sub domain", satisfied)
	}
	return s
}

// IsHttpsUrl adds a rule asserting that the string must be a valid HTTPS URL.
// Use AllowEmptyString() option to consider empty strings as valid.
func (s *String) IsHttpsUrl(opts ...StringValidationOption) *String {
	options := mergeStringValidationOptions(opts...)
	satisfied, err := regexp.MatchString(httpsUrlRgx, s.value)
	if err != nil {
		satisfied = false
	}
	if options.allowEmpty {
		if s.value == "" {
			satisfied = true
		}
		s.add("be empty or a valid https url", "is empty or a valid https url", satisfied)
	} else {
		s.add("be a valid https url", "is a valid https url", satisfied)
	}
	return s
}
