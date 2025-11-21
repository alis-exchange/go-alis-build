package validation

import (
	"regexp"
	"strings"
)

// List provides validation rules for list values.
type List[T any] struct {
	standard[[]T]
}

// newList creates and returns a new List[T] instance.
func newList[T any](path string, value []T) List[T] {
	return List[T]{standard: newStandard(path, value)}
}

// IsPopulated adds a rule asserting that the list must not be empty.
func (l *List[T]) IsPopulated() *List[T] {
	l.add("be populated", "is populated", len(l.value) > 0)
	return l
}

// IsEmpty adds a rule asserting that the list must be empty.
func (l *List[T]) IsEmpty() *List[T] {
	l.add("be empty", "is empty", len(l.value) == 0)
	return l
}

// LengthEq adds a rule asserting that the length of the list must be equal to the given length.
func (l *List[T]) LengthEq(eq int) *List[T] {
	l.add("have a length equal to %v", "has a length equal to %v", len(l.value) == eq, eq)
	return l
}

// LengthGt adds a rule asserting that the length of the list must be strictly greater than the given length.
func (l *List[T]) LengthGt(min int) *List[T] {
	l.add("have a length greater than %v", "has a length greater than %v", len(l.value) > min, min)
	return l
}

// LengthGte adds a rule asserting that the length of the list must be greater than or equal to the given length.
func (l *List[T]) LengthGte(min int) *List[T] {
	l.add("have a length greater than or equal to %v", "has a length greater than or equal to %v", len(l.value) >= min, min)
	return l
}

// LengthLt adds a rule asserting that the length of the list must be strictly less than the given length.
func (l *List[T]) LengthLt(max int) *List[T] {
	l.add("have a length less than %v", "has a length less than %v", len(l.value) < max, max)
	return l
}

// LengthLte adds a rule asserting that the length of the list must be less than or equal to the given length.
func (l *List[T]) LengthLte(max int) *List[T] {
	l.add("have a length less than or equal to %v", "has a length less than or equal to %v", len(l.value) <= max, max)
	return l
}

// StringList provides validation rules for lists of strings.
type StringList struct {
	List[string]
}

// Includes adds a rule asserting that the list must contain the given string.
func (l *StringList) Includes(value string) *StringList {
	satisfied := false
	for _, v := range l.value {
		if v == value {
			satisfied = true
			break
		}
	}
	l.add("include %v", "includes %v", satisfied, value)
	return l
}

// Excludes adds a rule asserting that the list must not contain the given string.
func (l *StringList) Excludes(value string) *StringList {
	satisfied := true
	for _, v := range l.value {
		if v == value {
			satisfied = false
			break
		}
	}
	l.add("exclude %v", "excludes %v", satisfied, value)
	return l
}

// EachUnique adds a rule asserting that all strings in the list must be unique.
func (l *StringList) EachUnique() *StringList {
	unique := make(map[string]bool)
	for _, v := range l.value {
		unique[v] = true
	}
	satisfied := len(unique) == len(l.value)
	l.add("have unique values", "values are unique", satisfied)
	return l
}

// EachPopulated adds a rule asserting that all strings in the list must be non-empty.
func (l *StringList) EachPopulated() *StringList {
	satisfied := true
	for _, v := range l.value {
		if v == "" {
			satisfied = false
			break
		}
	}
	l.add("have all values populated", "all values populated", satisfied)
	return l
}

// EachEq adds a rule asserting that all strings in the list must be equal to the given string.
func (l *StringList) EachEq(eq string) *StringList {
	satisfied := true
	for _, v := range l.value {
		if v != eq {
			satisfied = false
			break
		}
	}
	l.add("have all values equal to %v", "all values equal to %v", satisfied, eq)
	return l
}

// EachNotEq adds a rule asserting that all strings in the list must not be equal to the given string.
func (l *StringList) EachNotEq(neq string) *StringList {
	satisfied := true
	for _, v := range l.value {
		if v == neq {
			satisfied = false
			break
		}
	}
	l.add("have all values not equal to %v", "all values not equal to %v", satisfied, neq)
	return l
}

// EachOneof adds a rule asserting that all strings in the list must be one of the given values.
func (l *StringList) EachOneof(values ...string) *StringList {
	satisfied := true
	for _, v := range l.value {
		found := false
		for _, value := range values {
			if v == value {
				found = true
				break
			}
		}
		if !found {
			satisfied = false
			break
		}
	}
	l.add("have all values be one of %v", "all values are one of %v", satisfied, values)
	return l
}

// EachNoneof adds a rule asserting that all strings in the list must not be any of the given values.
func (l *StringList) EachNoneof(values ...string) *StringList {
	satisfied := true
	for _, v := range l.value {
		for _, value := range values {
			if v == value {
				satisfied = false
				break
			}
		}
	}
	l.add("have all values be none of %v", "all values are none of %v", satisfied, values)
	return l
}

// EachMatches adds a rule asserting that all strings in the list must match the given regular expression pattern.
func (l *StringList) EachMatches(pattern string) *StringList {
	satisfied := true
	for _, v := range l.value {
		if !regexp.MustCompile(pattern).MatchString(v) {
			satisfied = false
			break
		}
	}
	l.add("have all values match %v", "all values match %v", satisfied, pattern)
	return l
}

// EachMatchesOneof adds a rule asserting that all strings in the list must match at least one of the given regular expression patterns.
func (l *StringList) EachMatchesOneof(patterns ...string) *StringList {
	satisfied := true
	for _, v := range l.value {
		found := false
		for _, pattern := range patterns {
			if regexp.MustCompile(pattern).MatchString(v) {
				found = true
				break
			}
		}
		if !found {
			satisfied = false
			break
		}
	}
	l.add("have all values match one of %v", "all values match one of %v", satisfied, patterns)
	return l
}

// EachStartsWith adds a rule asserting that all strings in the list must start with the given prefix (or one of the additional prefixes).
func (l *StringList) EachStartsWith(prefix string, additionalPrefixes ...string) *StringList {
	prefixes := append(additionalPrefixes, prefix)
	allSatisfied := true
	for _, v := range l.value {
		satisfied := false
		for _, pref := range prefixes {
			if strings.HasPrefix(v, pref) {
				satisfied = true
				break
			}
		}
		if !satisfied {
			allSatisfied = false
			break
		}
	}
	l.add("have all values start with %v", "all values start with %v", allSatisfied, prefix)
	return l
}

// EachEndsWith adds a rule asserting that all strings in the list must end with the given suffix.
func (l *StringList) EachEndsWith(suffix string) *StringList {
	satisfied := true
	for _, v := range l.value {
		if !strings.HasSuffix(v, suffix) {
			satisfied = false
			break
		}
	}
	l.add("have all values end with %v", "all values end with %v", satisfied, suffix)
	return l
}

// EachContains adds a rule asserting that all strings in the list must contain the given substring.
func (l *StringList) EachContains(substr string) *StringList {
	satisfied := true
	for _, v := range l.value {
		if !strings.Contains(v, substr) {
			satisfied = false
			break
		}
	}
	l.add("have all values contain %v", "all values contain %v", satisfied, substr)
	return l
}

// EachIsEmail adds a rule asserting that all strings in the list must be valid email addresses.
func (s *StringList) EachIsEmail() *StringList {
	satisfied := true
	emailP, err := regexp.Compile(emailRgx)
	if err != nil {
		satisfied = false
	} else {
		for _, v := range s.value {
			if satisfied = emailP.MatchString(v); !satisfied {
				break
			}
		}
	}
	s.add("have all values be email addresses", "all values are email addresses", satisfied)
	return s
}

// NumberList provides validation rules for lists of numeric values.
type NumberList[T interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}] struct {
	List[T]
}

// EachUnique adds a rule asserting that all numbers in the list must be unique.
func (l *NumberList[T]) EachUnique() *NumberList[T] {
	unique := make(map[T]bool)
	for _, v := range l.value {
		unique[v] = true
	}
	satisfied := len(unique) == len(l.value)
	l.add("have unique values", "values are unique", satisfied)
	return l
}

// IsAscending adds a rule asserting that the numbers in the list must be in ascending order.
func (l *NumberList[T]) IsAscending() *NumberList[T] {
	satisfied := true
	for i := 1; i < len(l.value); i++ {
		if l.value[i] < l.value[i-1] {
			satisfied = false
			break
		}
	}
	l.add("be in ascending order", "is in ascending order", satisfied)
	return l
}

// IsDescending adds a rule asserting that the numbers in the list must be in descending order.
func (l *NumberList[T]) IsDescending() *NumberList[T] {
	satisfied := true
	for i := 1; i < len(l.value); i++ {
		if l.value[i] > l.value[i-1] {
			satisfied = false
			break
		}
	}
	l.add("be in descending order", "is in descending order", satisfied)
	return l
}

// EachPopulated adds a rule asserting that all numbers in the list must be non-zero.
func (l *NumberList[T]) EachPopulated() *NumberList[T] {
	satisfied := true
	for _, v := range l.value {
		if v == 0 {
			satisfied = false
			break
		}
	}
	l.add("have all values populated", "all values populated", satisfied)
	return l
}

// EachEq adds a rule asserting that all numbers in the list must be equal to the given number.
func (l *NumberList[T]) EachEq(eq T) *NumberList[T] {
	satisfied := true
	for _, v := range l.value {
		if v != eq {
			satisfied = false
			break
		}
	}
	l.add("have all values equal to %v", "all values equal to %v", satisfied, eq)
	return l
}

// EachNotEq adds a rule asserting that all numbers in the list must not be equal to the given number.
func (l *NumberList[T]) EachNotEq(neq T) *NumberList[T] {
	satisfied := true
	for _, v := range l.value {
		if v == neq {
			satisfied = false
			break
		}
	}
	l.add("have all values not equal to %v", "all values not equal to %v", satisfied, neq)
	return l
}

// EachOneof adds a rule asserting that all numbers in the list must be one of the given values.
func (l *NumberList[T]) EachOneof(values ...T) *NumberList[T] {
	satisfied := true
	for _, v := range l.value {
		found := false
		for _, value := range values {
			if v == value {
				found = true
				break
			}
		}
		if !found {
			satisfied = false
			break
		}
	}
	l.add("have all values be one of %v", "all values are one of %v", satisfied, values)
	return l
}

// EachNoneof adds a rule asserting that all numbers in the list must not be any of the given values.
func (l *NumberList[T]) EachNoneof(values ...T) *NumberList[T] {
	satisfied := true
	for _, v := range l.value {
		for _, value := range values {
			if v == value {
				satisfied = false
				break
			}
		}
	}
	l.add("have all values be none of %v", "all values are none of %v", satisfied, values)
	return l
}

// EachGt adds a rule asserting that all numbers in the list must be strictly greater than the given number.
func (l *NumberList[T]) EachGt(min T) *NumberList[T] {
	satisfied := true
	for _, v := range l.value {
		if v <= min {
			satisfied = false
			break
		}
	}
	l.add("have all values greater than %v", "all values are greater than %v", satisfied, min)
	return l
}

// EachGte adds a rule asserting that all numbers in the list must be greater than or equal to the given number.
func (l *NumberList[T]) EachGte(min T) *NumberList[T] {
	satisfied := true
	for _, v := range l.value {
		if v < min {
			satisfied = false
			break
		}
	}
	l.add("have all values greater than or equal to %v", "all values are greater than or equal to %v", satisfied, min)
	return l
}

// EachLt adds a rule asserting that all numbers in the list must be strictly less than the given number.
func (l *NumberList[T]) EachLt(max T) *NumberList[T] {
	satisfied := true
	for _, v := range l.value {
		if v >= max {
			satisfied = false
			break
		}
	}
	l.add("have all values less than %v", "all values are less than %v", satisfied, max)
	return l
}

// EachLte adds a rule asserting that all numbers in the list must be less than or equal to the given number.
func (l *NumberList[T]) EachLte(max T) *NumberList[T] {
	satisfied := true
	for _, v := range l.value {
		if v > max {
			satisfied = false
			break
		}
	}
	l.add("have all values less than or equal to %v", "all values are less than or equal to %v", satisfied, max)
	return l
}
