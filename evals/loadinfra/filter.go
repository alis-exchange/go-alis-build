package loadinfra

import (
	"strings"
)

// escapeFilterLabel escapes backslashes and double quotes for Monitoring filter
// label values embedded in double-quoted strings.
func escapeFilterLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
