package env

import (
	"fmt"
	"os"
)

// MustGet returns the value of the required environment variable with the given name.
// If the variable is not set, it panics with an error message.
func MustGet(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic("Required environment variable " + name + " is not set")
	}
	return value
}

// Get returns the value of the required environment variable with the given name.
// If the variable is not set it return an empty string
func Get(name string) string {
	return os.Getenv(name)
}

// LookupEnv retrieves the value of the environment variable named by the key.
// If the variable is present in the environment the value (which may be empty)
// is returned and the boolean is true. Otherwise the returned value will be empty
// and the boolean will be false.
func Lookup(name string) (string, bool) {
	return os.LookupEnv(name)
}

// MustExist checks if all provided environment variables are set.
// If any are missing, it panics with an error message detailing which ones.
func MustExist(names ...string) {
	var missing []string
	for _, name := range names {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf("missing required environment variables: %v", missing))
	}
}
