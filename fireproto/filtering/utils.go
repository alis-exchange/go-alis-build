package filtering

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/type/date"
)

// toCamelCase converts a snake_case string to camelCase
func toCamelCase(str string) string {
	link := regexp.MustCompile("(^[A-Za-z])|_([A-Za-z])")

	return link.ReplaceAllStringFunc(str, func(s string) string {
		return strings.ToUpper(strings.Replace(s, "_", "", -1))
	})
}

// validateArgument validates an argument against a regex
func validateArgument(name, value, regex string) error {
	if !regexp.MustCompile(regex).MatchString(value) {
		return fmt.Errorf("invalid %s: %s", name, value)
	}

	return nil
}

// ParseISOStringToDate parses a string in the format "YYYY-MM-DD" to a date.Date
func parseISOStringToDate(s string) (*date.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}

	return &date.Date{
		Year:  int32(t.Year()),
		Month: int32(t.Month()),
		Day:   int32(t.Day()),
	}, nil
}
