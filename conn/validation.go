package conn

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"regexp"
)

// validateArgument validates an argument and returns an error if not valid.
func validateArgument(name string, value string, regex string) error {
	// Validate the Name field using regex
	validateName := regexp.MustCompile(regex)
	validatedName := validateName.MatchString(value)
	if !validatedName {
		return status.Errorf(
			codes.InvalidArgument,
			"%s (%s) is not of the right format: %s", name, value, regex)
	}
	return nil
}
