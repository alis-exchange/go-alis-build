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
)

type String struct {
	standard[string]
}

func (s *String) IsPopulated() *String {
	s.add("be populated", "is populated", s.value != "")
	return s
}

func (s *String) IsEmpty() *String {
	s.add("be empty", "is empty", s.value == "")
	return s
}

func (s *String) Eq(eq string) *String {
	s.add("be equal to %v", "is equal to %v", s.value == eq, eq)
	return s
}

func (s *String) NotEq(neq string) *String {
	s.add("not be equal to %v", "is not equal to %v", s.value != neq, neq)
	return s
}

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

func (s *String) StartsWith(prefix string) *String {
	s.add("start with '%v'", "starts with '%v'", strings.HasPrefix(s.value, prefix), prefix)
	return s
}

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

func (s *String) EndsWith(suffix string) *String {
	s.add("end with %v", "ends with %v", strings.HasSuffix(s.value, suffix), suffix)
	return s
}

func (s *String) Contains(substr string) *String {
	s.add("contain %v", "contains %v", strings.Contains(s.value, substr), substr)
	return s
}

func (s *String) NotContains(substr string) *String {
	s.add("not contain %v", "does not contain %v", !strings.Contains(s.value, substr), substr)
	return s
}

func (s *String) LenEq(length int) *String {
	s.add("have length equal to %v", "has length equal to %v", len(s.value) == length, length)
	return s
}

func (s *String) LenGt(length int) *String {
	s.add("have length greater than %v", "has length greater than %v", len(s.value) > length, length)
	return s
}

func (s *String) LenGte(length int) *String {
	s.add("have length greater than or equal to %v", "has length greater than or equal to %v", len(s.value) >= length, length)
	return s
}

func (s *String) LenLt(length int) *String {
	s.add("have length less than %v", "has length less than %v", len(s.value) < length, length)
	return s
}

func (s *String) LenLte(length int) *String {
	s.add("have length less than or equal to %v", "has length less than or equal to %v", len(s.value) <= length, length)
	return s
}

func (s *String) Match(pattern string) *String {
	satisfied, err := regexp.MatchString(pattern, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("match %v", "matches %v", satisfied, pattern)
	return s
}

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

func (s *String) NotMatch(pattern string) *String {
	satisfied, err := regexp.MatchString(pattern, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("not match %v", "does not match %v", !satisfied, pattern)
	return s
}

func (s *String) IsEmail() *String {
	satisfied, err := regexp.MatchString(emailRgx, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("be a valid email", "is a valid email", satisfied)
	return s
}

func (s *String) IsDomain() *String {
	satisfied, err := regexp.MatchString(domainRgx, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("be a valid domain", "is a valid domain", satisfied)
	return s
}

func (s *String) IsRootDomain() *String {
	satisfied, err := regexp.MatchString(rootDomainRgx, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("be a valid root domain", "is a valid root domain", satisfied)
	return s
}

func (s *String) IsSubDomain() *String {
	satisfied, err := regexp.MatchString(subDomainRgx, s.value)
	if err != nil {
		satisfied = false
	}
	s.add("be a valid sub domain", "is a valid sub domain", satisfied)
	return s
}
