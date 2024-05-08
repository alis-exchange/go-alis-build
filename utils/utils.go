package utils

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
func Transform[T, U any](arr []T, fn func(T) U) []U {
	result := make([]U, len(arr))
	for i, v := range arr {
		result[i] = fn(v)
	}
	return result
}
