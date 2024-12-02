package sets

// Set is a generic set implementation.
// Items added are guaranteed to be unique.
type Set[T comparable] struct {
	elements map[T]bool
}

// NewSet initializes a new Set.
//
// Example:
//
//	set := NewSet(1, 2, 3)
//	set.Add(4)
func NewSet[T comparable](items ...T) *Set[T] {
	set := &Set[T]{elements: make(map[T]bool)}
	for _, item := range items {
		set.Add(item)
	}

	return set
}

// Add adds an element to the set.
func (s *Set[T]) Add(value T) {
	s.elements[value] = true
}

// Remove removes an element from the set.
func (s *Set[T]) Remove(value T) {
	delete(s.elements, value)
}

// Contains checks if the set contains an element.
func (s *Set[T]) Contains(value T) bool {
	_, exists := s.elements[value]
	return exists
}

// Values returns all elements in the set as a slice.
// The order of the elements is not guaranteed.
func (s *Set[T]) Values() []T {
	keys := make([]T, 0, len(s.elements))
	for key := range s.elements {
		keys = append(keys, key)
	}
	return keys
}

// Len returns the number of elements in the set.
func (s *Set[T]) Len() int {
	return len(s.elements)
}
