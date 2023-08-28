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
