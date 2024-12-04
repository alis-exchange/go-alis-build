package maps

import "sync"

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
