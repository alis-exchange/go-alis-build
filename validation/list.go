package validation

import (
	"regexp"
	"strings"
)

type List[T any] struct {
	standard[[]T]
}

func newList[T any](path string, value []T) List[T] {
	return List[T]{standard: newStandard(path, value)}
}

func (l *List[T]) IsPopulated() *List[T] {
	l.add("be populated", "is populated", len(l.value) > 0)
	return l
}

func (l *List[T]) LengthEq(eq int) *List[T] {
	l.add("have a length equal to %v", "has a length equal to %v", len(l.value) == eq, eq)
	return l
}

func (l *List[T]) LengthGt(min int) *List[T] {
	l.add("have a length greater than %v", "has a length greater than %v", len(l.value) > min, min)
	return l
}

func (l *List[T]) LengthGte(min int) *List[T] {
	l.add("have a length greater than or equal to %v", "has a length greater than or equal to %v", len(l.value) >= min, min)
	return l
}

func (l *List[T]) LengthLt(max int) *List[T] {
	l.add("have a length less than %v", "has a length less than %v", len(l.value) < max, max)
	return l
}

func (l *List[T]) LengthLte(max int) *List[T] {
	l.add("have a length less than or equal to %v", "has a length less than or equal to %v", len(l.value) <= max, max)
	return l
}

type StringList struct {
	List[string]
}

func (l *StringList) EachUnique() *StringList {
	unique := make(map[string]bool)
	for _, v := range l.value {
		unique[v] = true
	}
	satisfied := len(unique) == len(l.value)
	l.add("have unique values", "values are unique", satisfied)
	return l
}

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

func (l *StringList) EachStartsWith(prefix string) *StringList {
	satisfied := true
	for _, v := range l.value {
		if !strings.HasPrefix(v, prefix) {
			satisfied = false
			break
		}
	}
	l.add("have all values start with %v", "all values start with %v", satisfied, prefix)
	return l
}

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

type NumberList[T interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}] struct {
	List[T]
}

func (l *NumberList[T]) EachUnique() *NumberList[T] {
	unique := make(map[T]bool)
	for _, v := range l.value {
		unique[v] = true
	}
	satisfied := len(unique) == len(l.value)
	l.add("have unique values", "values are unique", satisfied)
	return l
}

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

func (l *NumberList[T]) EachNeq(neq T) *NumberList[T] {
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
