package utils

import (
	"errors"
	"math/rand"
	"sync"
	"time"
)

// Contains function is a nifty tool! It returns true if it finds the value you're looking for in the array.
// And guess what? It can handle any type, all thanks to the power of Go's Generics!
func Contains[T comparable](s []T, searchTerm T) bool {
	for _, item := range s {
		if item == searchTerm {
			return true
		}
	}
	return false
}

// Transform is a utility function for transforming the elements of a slice.
//
// For example, you can use it to double the values of a slice of integers:
//
//	ints := []int{1, 2, 3}
//	doubled := Transform(ints, func(i int) int { return i * 2 })
//	// doubled = [2, 4, 6]
func Transform[T, U any](arr []T, fn func(T) U) []U {
	result := make([]U, len(arr))
	for i, v := range arr {
		result[i] = fn(v)
	}
	return result
}

// Find is a utility function for finding the first element in a slice that satisfies a given predicate.
//
// It takes a slice and a predicate function as arguments. The predicate function should return true if the
// element satisfies the condition, and false otherwise.
//
// It returns the element, its index, and a boolean indicating whether the element was found.
// If the element is not found, it returns the zero value of type T, -1, and false.
//
// For example, you can use it to find the first even number in a slice of integers:
//
//	ints := []int{1, 2, 3, 4, 5}
//	even, index, found := Find(ints, func(i int) bool { return i%2 == 0 })
//	// even = 2, index = 1, found = true
func Find[T any](arr []T, fn func(T) bool) (T, int, bool) {
	var zero T // zero value of type T
	for i, v := range arr {
		if fn(v) {
			return v, i, true
		}
	}
	return zero, -1, false
}

// Filter is a utility function for filtering elements that satisfy a given predicate from a slice.
//
// It takes a slice and a predicate function as arguments. The predicate function should return true if the
// element satisfies the condition, and false otherwise.
//
// It returns a new slice containing only the elements that satisfy the condition.
func Filter[T any](arr []T, fn func(T) bool) []T {
	var result []T
	for _, v := range arr {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

// Reduce is a utility function for reducing a slice to a single value.
//
// It takes a slice, a reducer function, and an initial value as arguments. The reducer function should take
// two arguments of the same type as the elements of the slice and return a single value of the same type.
//
// It returns the final reduced value.
func Reduce[T any, R any](arr []T, fn func(R, T) R, initial R) R {
	result := initial
	for _, v := range arr {
		result = fn(result, v)
	}
	return result
}

// Chunk is a utility function for splitting a slice into chunks of a given size.
//
// It takes a slice and a chunk size as arguments and returns a slice of slices, where each slice has at most
// the given chunk size.
//
// For example, you can use it to split a slice of integers into chunks of size 2:
//
//	ints := []int{1, 2, 3, 4, 5}
//	chunks := Chunk(ints, 2)
//	// chunks = [[1, 2], [3, 4], [5]]
func Chunk[T any](arr []T, size int) [][]T {
	if size <= 0 {
		return nil
	}
	var result [][]T
	for size < len(arr) {
		arr, result = arr[size:], append(result, arr[0:size:size])
	}
	result = append(result, arr)
	return result
}

// Unique is a utility function for removing duplicate elements from a slice.
//
// It takes a slice as an argument and returns a new slice containing only the unique elements in the original slice.
//
// The elements in the slice must be comparable.
//
// For example, you can use it to remove duplicate integers from a slice:
//
//	ints := []int{1, 2, 2, 3, 3, 3}
//	uniqueInts := Unique(ints)
//	// uniqueInts = [1, 2, 3]
func Unique[T comparable](arr []T) []T {
	m := make(map[T]bool)
	var result []T
	for _, v := range arr {
		if !m[v] {
			m[v] = true
			result = append(result, v)
		}
	}
	return result
}

// GroupBy is a utility function for grouping elements of a slice by a key function.
//
// It takes a slice and a key function as arguments and returns a map where the keys are the result of applying
// the key function to the elements of the slice, and the values are slices of elements that have the same key.
//
// For example, you can use it to group a slice of integers by their parity:
//
//	ints := []int{1, 2, 3, 4, 5}
//	grouped := GroupBy(ints, func(i int) string {
//		if i%2 == 0 {
//			return "even"
//		}
//		return "odd"
//	})
//	// grouped = map[string][]int{"even": [2, 4], "odd": [1, 3, 5]}
func GroupBy[T any, K comparable](arr []T, fn func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, v := range arr {
		key := fn(v)
		result[key] = append(result[key], v)
	}
	return result
}

// OrderedMap structure that maintains the insertion order
type OrderedMap[K comparable, V any] struct {
	keys   []K
	values map[K]V
	mu     sync.RWMutex // Mutex to ensure concurrency safety
}

/*
NewOrderedMap creates a new instance of OrderedMap

To ensure concurrency safety and avoid race conditions and deadlocks,
avoid cross-instance dependencies. Ensure that operations on different OrderedMap instances are
independent and do not inadvertently depend on each other.
Take the following example:

	var o1, o2 OrderedMap[int, string]
	go func() {
		o1.Set(1, "one")
		o2.Get(1)  // Waits on o1 indirectly
	}()

	go func() {
		o2.Set(2, "two")
		o1.Get(2)  // Waits on o2 indirectly
	}()

One should also be careful when using the Range method. The callback function should not modify the OrderedMap.
*/
func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{
		keys:   []K{},
		values: make(map[K]V),
	}
}

// Len returns the number of key-value pairs in the OrderedMap
func (o *OrderedMap[K, V]) Len() int {
	return len(o.keys)
}

// Clear removes all key-value pairs from the OrderedMap
func (o *OrderedMap[K, V]) Clear() {
	o.keys = []K{}
	o.values = make(map[K]V)
}

// Set adds or updates a key-value pair in the OrderedMap
func (o *OrderedMap[K, V]) Set(key K, value V) {
	o.mu.Lock()         // Lock for writing
	defer o.mu.Unlock() // Unlock after operation

	// Check if the key already exists in the map
	if _, exists := o.values[key]; !exists {
		// If the key does not exist, add it to the keys slice
		o.keys = append(o.keys, key)
	}
	// Set the value in the map
	o.values[key] = value
}

// Get retrieves the value associated with a key
func (o *OrderedMap[K, V]) Get(key K) (V, bool) {
	o.mu.RLock()         // Lock for reading
	defer o.mu.RUnlock() // Unlock after operation

	value, exists := o.values[key]
	return value, exists
}

// Delete removes a key-value pair from the OrderedMap
func (o *OrderedMap[K, V]) Delete(key K) {
	o.mu.Lock()         // Lock for writing
	defer o.mu.Unlock() // Unlock after operation

	if _, exists := o.values[key]; exists {
		// Delete the key from the map
		delete(o.values, key)
		// Remove the key from the keys slice
		for i, k := range o.keys {
			if k == key {
				o.keys = append(o.keys[:i], o.keys[i+1:]...)
				break
			}
		}
	}
}

// Keys returns the keys in the order they were added
func (o *OrderedMap[K, V]) Keys() []K {
	o.mu.RLock()         // Lock for reading
	defer o.mu.RUnlock() // Unlock after operation

	// Return a copy of the keys slice to prevent modification
	return append([]K(nil), o.keys...)
}

// Values returns the values in the order they were added
func (o *OrderedMap[K, V]) Values() []V {
	o.mu.RLock()         // Lock for reading
	defer o.mu.RUnlock() // Unlock after operation

	orderedValues := make([]V, len(o.keys))
	for i, key := range o.keys {
		orderedValues[i] = o.values[key]
	}
	return orderedValues
}

// Range iterates over the OrderedMap in the order of insertion and applies a callback function.
// If the callback function returns false, the iteration stops.
//
// The callback function should not modify the OrderedMap in any way.
// For example: Calling Set, Delete, or Clear inside the callback function will cause a deadlock.
func (o *OrderedMap[K, V]) Range(cb func(int, K, V) bool) {
	o.mu.RLock()         // Lock for reading
	defer o.mu.RUnlock() // Unlock after operation

	// Iterate over the keys and make a shallow copy of each key-value pair
	for i, key := range o.keys {
		value := o.values[key]
		// Pass the copy of the key-value pair to the callback
		if !cb(i, key, value) {
			break
		}
	}
}

// Retry is a utility function to retry a function a number of times with exponential backoff
// and jitter. It will return the result of the function if it succeeds, or the last error if
// it fails.
//
// If the error returned inside Retry is a NonRetryableError, it will stop retrying and
// return the original error for later checking.
func Retry[R interface{}](attempts int, baseSleep time.Duration, f func() (R, error)) (R, error) {
	if res, err := f(); err != nil {
		var s NonRetryableError
		if errors.As(err, &s) {
			// Return the original error for later checking
			return res, s.error
		}

		if attempts--; attempts > 0 {
			// Calculate exponential backoff
			// This multiplies the base sleep time by 2 raised to the power of the remaining attempts,
			// which increases the sleep duration exponentially as the number of attempts decreases.
			sleep := baseSleep * (1 << uint(attempts))

			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			time.Sleep(sleep)
			return Retry[R](attempts, baseSleep, f)
		}
		return res, err
	} else {
		return res, nil
	}
}

// NonRetryableError is a utility type to return an error that will not be retried by Retry
type NonRetryableError struct {
	error
}

// NewNonRetryableError is a utility function to return a NonRetryableError
func NewNonRetryableError(err error) NonRetryableError {
	return NonRetryableError{err}
}
