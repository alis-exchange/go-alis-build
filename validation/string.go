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

type String struct {
	standard[string]
}

// Adds a rule to the parent validator asserting that the string value is populated.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) IsPopulated() *String {
	s.add("be populated", "is populated", s.value != "")
	return s
}

// Adds a rule to the parent validator asserting that the string value is empty.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) IsEmpty() *String {
	s.add("be empty", "is empty", s.value == "")
	return s
}

// Adds a rule to the parent validator asserting that the string value is equal to the given value.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) Eq(eq string) *String {
	s.add("be equal to %v", "is equal to %v", s.value == eq, eq)
	return s
}

// Adds a rule to the parent validator asserting that the string value is not equal to the given value.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) NotEq(neq string) *String {
	s.add("not be equal to %v", "is not equal to %v", s.value != neq, neq)
	return s
}

// Adds a rule to the parent validator asserting that the string value is one of the given values.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
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

// Adds a rule to the parent validator asserting that the string value is none of the given values.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
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

// Adds a rule to the parent validator asserting that the string value starts with the given prefix.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) StartsWith(prefix string) *String {
	s.add("start with '%v'", "starts with '%v'", strings.HasPrefix(s.value, prefix), prefix)
	return s
}

// Adds a rule to the parent validator asserting that the string value starts with one of the given prefixes.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
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

// Adds a rule to the parent validator asserting that the string value starts with none of the given prefixes.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
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

// Adds a rule to the parent validator asserting that the string value ends with the given suffix.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) EndsWith(suffix string) *String {
	s.add("end with %v", "ends with %v", strings.HasSuffix(s.value, suffix), suffix)
	return s
}

// Adds a rule to the parent validator asserting that the string value contains the given substring.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) Contains(substr string) *String {
	s.add("contain %v", "contains %v", strings.Contains(s.value, substr), substr)
	return s
}

// Adds a rule to the parent validator asserting that the string value does not contain the given substring.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) NotContains(substr string) *String {
	s.add("not contain %v", "does not contain %v", !strings.Contains(s.value, substr), substr)
	return s
}

// Adds a rule to the parent validator asserting that the string value has a length equal to the given length.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) LenEq(length int) *String {
	s.add("have length equal to %v", "has length equal to %v", len(s.value) == length, length)
	return s
}

// Adds a rule to the parent validator asserting that the string value has a length greater than the given length.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) LenGt(length int) *String {
	s.add("have length greater than %v", "has length greater than %v", len(s.value) > length, length)
	return s
}

// Adds a rule to the parent validator asserting that the string value has a length greater than or equal to the given length.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) LenGte(length int) *String {
	s.add("have length greater than or equal to %v", "has length greater than or equal to %v", len(s.value) >= length, length)
	return s
}

// Adds a rule to the parent validator asserting that the string value has a length less than the given length.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) LenLt(length int) *String {
	s.add("have length less than %v", "has length less than %v", len(s.value) < length, length)
	return s
}

// Adds a rule to the parent validator asserting that the string value has a length less than or equal to the given length.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) LenLte(length int) *String {
	s.add("have length less than or equal to %v", "has length less than or equal to %v", len(s.value) <= length, length)
	return s
}

// Adds a rule to the parent validator asserting that the string value matches the given pattern.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) Matches(pattern string) *String {
	satisfied, err := regexp.MatchString(pattern, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("match %v", "matches %v", satisfied, pattern)
	return s
}

// Adds a rule to the parent validator asserting that the string value matches one of the given patterns.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
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

// Adds a rule to the parent validator asserting that the string value matches none of the given patterns.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
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

// Adds a rule to the parent validator asserting that the string value does not match the given pattern.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) NotMatch(pattern string) *String {
	satisfied, err := regexp.MatchString(pattern, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("not match %v", "does not match %v", !satisfied, pattern)
	return s
}

// Adds a rule to the parent validator asserting that the string value is a valid email.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) IsEmail() *String {
	satisfied, err := regexp.MatchString(emailRgx, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("be a valid email", "is a valid email", satisfied)
	return s
}

// Adds a rule to the parent validator asserting that the string value is a valid domain.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) IsDomain() *String {
	satisfied, err := regexp.MatchString(domainRgx, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("be a valid domain", "is a valid domain", satisfied)
	return s
}

// Adds a rule to the parent validator asserting that the string value is a valid root domain.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) IsRootDomain() *String {
	satisfied, err := regexp.MatchString(rootDomainRgx, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("be a valid root domain", "is a valid root domain", satisfied)
	return s
}

// Adds a rule to the parent validator asserting that the string value is a valid sub domain.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) IsSubDomain() *String {
	satisfied, err := regexp.MatchString(subDomainRgx, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("be a valid sub domain", "is a valid sub domain", satisfied)
	return s
}

// Adds a rule to the parent validator asserting that the string value is a valid https url.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (s *String) IsHttpsDomain() *String {
	satisfied, err := regexp.MatchString(httpsUrlRgx, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("be a valid https domain", "is a valid https domain", satisfied)
	return s
}
