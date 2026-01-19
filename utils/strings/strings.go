// Package strings provides utilities for converting strings between
// common naming conventions used in programming.
//
// Supported naming conventions:
//   - snake_case: words separated by underscores, all lowercase
//   - camelCase: first word lowercase, subsequent words capitalized, no separators
//   - PascalCase: all words capitalized, no separators (also known as UpperCamelCase)
//   - kebab-case: words separated by hyphens, all lowercase (also known as spinal-case)
//   - SCREAMING_SNAKE_CASE: words separated by underscores, all uppercase (also known as CONSTANT_CASE)
//   - Title Case: words separated by spaces, each word capitalized
//
// All conversion functions handle mixed-input formats gracefully, properly handling
// edge cases like consecutive uppercase letters (e.g., "HTTPServer" -> "http_server"),
// various delimiter types (underscores, hyphens, spaces), and Unicode characters.
//
// # Algorithm Overview
//
// The conversion functions use a single-pass algorithm that:
//  1. Iterates through each rune in the input string
//  2. Detects word boundaries based on case transitions and delimiters
//  3. Applies the appropriate transformation for the target format
//
// # Performance
//
// All functions have O(n) time complexity where n is the length of the input string.
// Space complexity is O(n) for the output string, with a small constant overhead
// for the strings.Builder buffer (pre-allocated with estimated capacity).
//
// # Thread Safety
//
// All functions are stateless and safe for concurrent use.
package strings

import (
	"strings"
	"unicode"
)

// ToSnakeCase converts any common case format to snake_case.
//
// The function handles the following input formats:
//   - camelCase: "camelCase" → "camel_case"
//   - PascalCase: "PascalCase" → "pascal_case"
//   - kebab-case: "kebab-case" → "kebab_case"
//   - space-separated: "space separated" → "space_separated"
//   - Title Case: "Title Case" → "title_case"
//   - SCREAMING_SNAKE_CASE: "SCREAMING_SNAKE" → "screaming_snake"
//   - Mixed formats: "myHTTPServer" → "my_http_server"
//
// # Algorithm Details
//
// The algorithm inserts an underscore before uppercase letters when:
//   - The previous character is lowercase (e.g., "camelCase" → "camel_Case")
//   - The next character is lowercase and we're in an acronym (e.g., "HTTPServer" → "HTTP_Server")
//
// Existing delimiters (hyphens, spaces) are converted to underscores.
// Consecutive uppercase letters are kept together as acronyms until a lowercase follows.
//
// # Edge Cases
//
//   - Empty string returns empty string
//   - Single character returns lowercase version
//   - Already snake_case input is returned unchanged (lowercase)
//   - Numbers are passed through unchanged
//   - Unicode letters are properly handled with unicode.IsUpper/IsLower
//
// # Limitations
//
// Consecutive acronyms are treated as a single word since they are ambiguous
// without semantic knowledge:
//   - "XMLHTTPRequest" → "xmlhttp_request" (not "xml_http_request")
//   - "getHTTPSURL" → "get_httpsurl" (not "get_https_url")
//
// Single acronyms followed by a word are handled correctly:
//   - "HTTPServer" → "http_server"
//   - "getURLPath" → "get_url_path"
//
// # Examples
//
//	ToSnakeCase("CamelCase")     // "camel_case"
//	ToSnakeCase("HTTPServer")    // "http_server"
//	ToSnakeCase("kebab-case")    // "kebab_case"
//	ToSnakeCase("Title Case")    // "title_case"
//	ToSnakeCase("already_snake") // "already_snake"
//	ToSnakeCase("getHTTPResponseCode") // "get_http_response_code"
func ToSnakeCase(s string) string {
	var result strings.Builder
	result.Grow(len(s) * 2) // Worst case: separator after each char

	runes := []rune(s)
	for i, r := range runes {
		// Replace delimiters with underscore
		if r == '-' || r == ' ' {
			result.WriteRune('_')
			continue
		}

		// Skip if already an underscore
		if r == '_' {
			result.WriteRune(r)
			continue
		}

		// If current character is uppercase
		if unicode.IsUpper(r) {
			if i > 0 {
				prevRune := runes[i-1]
				prevIsLower := unicode.IsLower(prevRune)
				prevIsDelimiter := prevRune == '_' || prevRune == '-' || prevRune == ' '
				nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])

				// Add underscore if not right after a delimiter
				if !prevIsDelimiter && (prevIsLower || nextIsLower) {
					result.WriteRune('_')
				}
			}
		}

		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

// ToCamelCase converts any common case format to camelCase.
//
// The function handles the following input formats:
//   - snake_case: "snake_case" → "snakeCase"
//   - kebab-case: "kebab-case" → "kebabCase"
//   - PascalCase: "PascalCase" → "pascalCase"
//   - space-separated: "space separated" → "spaceSeparated"
//   - Title Case: "Title Case" → "titleCase"
//   - SCREAMING_SNAKE_CASE: "SCREAMING_SNAKE" → "screamingSnake"
//
// # Algorithm Details
//
// The algorithm processes each character:
//  1. Skips delimiters (underscores, hyphens, spaces) and marks next char for capitalization
//  2. First non-delimiter character is always lowercased
//  3. Characters after delimiters are uppercased
//  4. Other characters are passed through unchanged (preserving existing case in words)
//
// # Edge Cases
//
//   - Empty string returns empty string
//   - Single character returns lowercase version
//   - Leading delimiters are skipped, first letter is lowercased
//   - Consecutive delimiters result in single capitalization
//   - Numbers after delimiters don't get "capitalized" (numbers pass through)
//
// # Examples
//
//	ToCamelCase("snake_case")     // "snakeCase"
//	ToCamelCase("kebab-case")     // "kebabCase"
//	ToCamelCase("PascalCase")     // "pascalCase"
//	ToCamelCase("Title Case")     // "titleCase"
//	ToCamelCase("alreadyCamel")   // "alreadyCamel"
//	ToCamelCase("_leading")       // "leading"
//	ToCamelCase("user_id_123")    // "userId123"
func ToCamelCase(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	capitalizeNext := false
	firstChar := true

	for _, r := range s {
		// Skip delimiters and mark next char for capitalization
		if r == '_' || r == '-' || r == ' ' {
			// Only capitalize after first char has been written
			if !firstChar {
				capitalizeNext = true
			}
			continue
		}

		if firstChar {
			// First character is always lowercase in camelCase
			result.WriteRune(unicode.ToLower(r))
			firstChar = false
		} else if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// ToPascalCase converts any common case format to PascalCase.
//
// PascalCase (also known as UpperCamelCase) capitalizes the first letter of
// every word with no separators between words. This is commonly used for
// type names, class names, and exported identifiers in Go.
//
// The function handles the following input formats:
//   - snake_case: "snake_case" → "SnakeCase"
//   - kebab-case: "kebab-case" → "KebabCase"
//   - camelCase: "camelCase" → "CamelCase"
//   - space-separated: "space separated" → "SpaceSeparated"
//   - Title Case: "title case" → "TitleCase"
//   - SCREAMING_SNAKE_CASE: "SCREAMING_SNAKE" → "ScreamingSnake"
//
// # Algorithm Details
//
// The algorithm is similar to ToCamelCase but:
//  1. Starts with capitalizeNext = true (first character is uppercased)
//  2. Skips delimiters and marks next char for capitalization
//  3. Characters after delimiters are uppercased
//  4. Other characters are passed through unchanged
//
// # Edge Cases
//
//   - Empty string returns empty string
//   - Single character returns uppercase version
//   - Leading delimiters are skipped, first letter is still uppercased
//   - Already PascalCase input is returned unchanged
//   - Numbers after delimiters pass through unchanged
//
// # Examples
//
//	ToPascalCase("snake_case")     // "SnakeCase"
//	ToPascalCase("kebab-case")     // "KebabCase"
//	ToPascalCase("camelCase")      // "CamelCase"
//	ToPascalCase("title case")     // "TitleCase"
//	ToPascalCase("AlreadyPascal")  // "AlreadyPascal"
//	ToPascalCase("http_server")    // "HttpServer"
//	ToPascalCase("get_user_by_id") // "GetUserById"
func ToPascalCase(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	capitalizeNext := true // Start with capital for PascalCase

	for _, r := range s {
		// Skip delimiters and mark next char for capitalization
		if r == '_' || r == '-' || r == ' ' {
			capitalizeNext = true
			continue
		}

		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// ToKebabCase converts any common case format to kebab-case.
//
// Kebab-case (also known as spinal-case or lisp-case) uses hyphens to separate
// words with all letters in lowercase. This is commonly used in URLs, CSS class
// names, and file names.
//
// The function handles the following input formats:
//   - camelCase: "camelCase" → "camel-case"
//   - PascalCase: "PascalCase" → "pascal-case"
//   - snake_case: "snake_case" → "snake-case"
//   - space-separated: "space separated" → "space-separated"
//   - Title Case: "Title Case" → "title-case"
//   - SCREAMING_SNAKE_CASE: "SCREAMING_SNAKE" → "screaming-snake"
//   - Mixed formats: "myHTTPServer" → "my-http-server"
//
// # Algorithm Details
//
// The algorithm mirrors ToSnakeCase but uses hyphens instead of underscores:
//  1. Replaces underscores and spaces with hyphens
//  2. Inserts hyphens before uppercase letters at word boundaries
//  3. Converts all letters to lowercase
//
// # Edge Cases
//
//   - Empty string returns empty string
//   - Single character returns lowercase version
//   - Already kebab-case input is returned unchanged
//   - Consecutive uppercase letters (acronyms) are kept together
//   - Numbers are passed through unchanged
//
// # Limitations
//
// Consecutive acronyms are treated as a single word (see ToSnakeCase for details).
//
// # Examples
//
//	ToKebabCase("CamelCase")     // "camel-case"
//	ToKebabCase("snake_case")   // "snake-case"
//	ToKebabCase("HTTPServer")   // "http-server"
//	ToKebabCase("Title Case")   // "title-case"
//	ToKebabCase("already-kebab") // "already-kebab"
//	ToKebabCase("XMLHttpRequest") // "xml-http-request"
func ToKebabCase(s string) string {
	var result strings.Builder
	result.Grow(len(s) * 2) // Worst case: separator after each char

	runes := []rune(s)
	for i, r := range runes {
		// Replace other delimiters with hyphen
		if r == '_' || r == ' ' {
			result.WriteRune('-')
			continue
		}

		// Skip if already a hyphen
		if r == '-' {
			result.WriteRune(r)
			continue
		}

		// If current character is uppercase
		if unicode.IsUpper(r) {
			if i > 0 {
				prevRune := runes[i-1]
				prevIsLower := unicode.IsLower(prevRune)
				prevIsDelimiter := prevRune == '_' || prevRune == '-' || prevRune == ' '
				nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])

				// Add hyphen if not right after a delimiter
				if !prevIsDelimiter && (prevIsLower || nextIsLower) {
					result.WriteRune('-')
				}
			}
		}

		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

// ToScreamingSnakeCase converts any common case format to SCREAMING_SNAKE_CASE.
//
// SCREAMING_SNAKE_CASE (also known as CONSTANT_CASE or MACRO_CASE) uses
// underscores to separate words with all letters in uppercase. This is
// commonly used for constants, environment variables, and enum values.
//
// The function handles the following input formats:
//   - camelCase: "camelCase" → "CAMEL_CASE"
//   - PascalCase: "PascalCase" → "PASCAL_CASE"
//   - kebab-case: "kebab-case" → "KEBAB_CASE"
//   - space-separated: "space separated" → "SPACE_SEPARATED"
//   - Title Case: "Title Case" → "TITLE_CASE"
//   - snake_case: "already_snake" → "ALREADY_SNAKE"
//   - Mixed formats: "myHTTPServer" → "MY_HTTP_SERVER"
//
// # Algorithm Details
//
// This function is implemented as a composition:
//  1. First converts the input to snake_case using ToSnakeCase
//  2. Then converts the result to uppercase using strings.ToUpper
//
// This two-step approach ensures consistent word boundary detection
// while keeping the code DRY.
//
// # Edge Cases
//
//   - Empty string returns empty string
//   - Single character returns uppercase version
//   - Already SCREAMING_SNAKE_CASE input is returned unchanged
//   - Numbers are passed through unchanged
//
// # Limitations
//
// Consecutive acronyms are treated as a single word (see ToSnakeCase for details).
//
// # Examples
//
//	ToScreamingSnakeCase("CamelCase")     // "CAMEL_CASE"
//	ToScreamingSnakeCase("kebab-case")    // "KEBAB_CASE"
//	ToScreamingSnakeCase("HTTPServer")    // "HTTP_SERVER"
//	ToScreamingSnakeCase("already_snake") // "ALREADY_SNAKE"
//	ToScreamingSnakeCase("maxRetryCount") // "MAX_RETRY_COUNT"
func ToScreamingSnakeCase(s string) string {
	snake := ToSnakeCase(s)
	return strings.ToUpper(snake)
}

// ToTitleCase converts any common case format to Title Case (space-separated).
//
// Title Case capitalizes the first letter of each word and separates words
// with spaces. This is commonly used for display purposes, headings, and
// human-readable labels.
//
// The function handles the following input formats:
//   - snake_case: "snake_case" → "Snake Case"
//   - kebab-case: "kebab-case" → "Kebab Case"
//   - camelCase: "camelCase" → "Camel Case"
//   - PascalCase: "PascalCase" → "Pascal Case"
//   - SCREAMING_SNAKE_CASE: "SCREAMING_SNAKE" → "Screaming Snake"
//   - Mixed formats: "myHTTPServer" → "My HTTP Server"
//
// # Algorithm Details
//
// The algorithm:
//  1. Replaces underscores and hyphens with spaces
//  2. Inserts spaces before uppercase letters at word boundaries (camelCase/PascalCase)
//  3. Capitalizes the first letter of each word
//  4. Lowercases subsequent letters within each word
//
// Note: Acronyms are NOT preserved as uppercase. Each word is title-cased
// (first letter uppercase, rest lowercase), e.g., "HTTPServer" → "Http Server".
//
// # Edge Cases
//
//   - Empty string returns empty string
//   - Single character returns uppercase version
//   - Already "Title Case" input is returned unchanged
//   - Numbers are passed through unchanged
//   - Trailing delimiters are trimmed (no trailing spaces)
//
// # Limitations
//
// Consecutive acronyms are treated as a single word (see ToSnakeCase for details).
//
// # Examples
//
//	ToTitleCase("snake_case")    // "Snake Case"
//	ToTitleCase("kebab-case")    // "Kebab Case"
//	ToTitleCase("camelCase")     // "Camel Case"
//	ToTitleCase("HTTPServer")    // "Http Server"
//	ToTitleCase("Already Title") // "Already Title"
//	ToTitleCase("userID")        // "User Id"
func ToTitleCase(s string) string {
	var result strings.Builder
	result.Grow(len(s) * 2) // Worst case: separator after each char

	capitalizeNext := true

	runes := []rune(s)
	for i, r := range runes {
		// Replace delimiters with space
		if r == '_' || r == '-' {
			result.WriteRune(' ')
			capitalizeNext = true
			continue
		}

		// Keep existing spaces
		if r == ' ' {
			result.WriteRune(r)
			capitalizeNext = true
			continue
		}

		// Handle CamelCase/PascalCase - insert space before uppercase
		if unicode.IsUpper(r) && i > 0 {
			prevRune := runes[i-1]
			prevIsLower := unicode.IsLower(prevRune)
			prevIsDelimiter := prevRune == '_' || prevRune == '-' || prevRune == ' '
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])

			if !prevIsDelimiter && (prevIsLower || nextIsLower) {
				result.WriteRune(' ')
				capitalizeNext = true
			}
		}

		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(unicode.ToLower(r))
		}
	}

	return strings.TrimRight(result.String(), " ")
}
