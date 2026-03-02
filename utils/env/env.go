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