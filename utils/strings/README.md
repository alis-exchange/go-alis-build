# Strings

The `strings` package provides utilities for converting strings between common naming conventions used in programming. It handles conversions between snake_case, camelCase, PascalCase, kebab-case, SCREAMING_SNAKE_CASE, and Title Case.

## Installation

Get the package

```bash
go get go.alis.build/utils
```

Import the package

```go
import "go.alis.build/utils/strings"
```

## Overview

This package provides functions to:
- Convert between different naming conventions (snake_case, camelCase, PascalCase, etc.)
- Handle edge cases like consecutive uppercase letters (acronyms)
- Process mixed-format inputs gracefully
- Support Unicode characters

All conversion functions are:
- **Stateless**: Safe for concurrent use
- **O(n) time complexity**: Single-pass algorithms
- **O(n) space complexity**: Output string with minimal overhead

## Functions

### ToSnakeCase

Converts any common case format to snake_case.

**Signature:**
```go
func ToSnakeCase(s string) string
```

**Behavior:**
- Inserts underscores before uppercase letters at word boundaries
- Converts all letters to lowercase
- Replaces hyphens and spaces with underscores
- Handles consecutive uppercase letters (acronyms) intelligently

**Example:**
```go
strings.ToSnakeCase("camelCase")      // "camel_case"
strings.ToSnakeCase("PascalCase")     // "pascal_case"
strings.ToSnakeCase("HTTPServer")     // "http_server"
strings.ToSnakeCase("kebab-case")     // "kebab_case"
strings.ToSnakeCase("Title Case")     // "title_case"
strings.ToSnakeCase("already_snake")  // "already_snake"
strings.ToSnakeCase("getHTTPResponse") // "get_http_response"
```

### ToCamelCase

Converts any common case format to camelCase.

**Signature:**
```go
func ToCamelCase(s string) string
```

**Behavior:**
- First character is always lowercased
- Characters following delimiters are uppercased
- Delimiters (underscores, hyphens, spaces) are removed
- Preserves existing case within words

**Example:**
```go
strings.ToCamelCase("snake_case")     // "snakeCase"
strings.ToCamelCase("kebab-case")     // "kebabCase"
strings.ToCamelCase("PascalCase")     // "pascalCase"
strings.ToCamelCase("Title Case")     // "titleCase"
strings.ToCamelCase("http_server")    // "httpServer"
strings.ToCamelCase("user_id_123")    // "userId123"
```

### ToPascalCase

Converts any common case format to PascalCase (also known as UpperCamelCase).

**Signature:**
```go
func ToPascalCase(s string) string
```

**Behavior:**
- First character is always uppercased
- Characters following delimiters are uppercased
- Delimiters (underscores, hyphens, spaces) are removed
- Preserves existing case within words

**Example:**
```go
strings.ToPascalCase("snake_case")     // "SnakeCase"
strings.ToPascalCase("kebab-case")     // "KebabCase"
strings.ToPascalCase("camelCase")      // "CamelCase"
strings.ToPascalCase("title case")     // "TitleCase"
strings.ToPascalCase("http_server")    // "HttpServer"
strings.ToPascalCase("get_user_by_id") // "GetUserById"
```

### ToKebabCase

Converts any common case format to kebab-case (also known as spinal-case).

**Signature:**
```go
func ToKebabCase(s string) string
```

**Behavior:**
- Inserts hyphens before uppercase letters at word boundaries
- Converts all letters to lowercase
- Replaces underscores and spaces with hyphens
- Handles consecutive uppercase letters (acronyms) intelligently

**Example:**
```go
strings.ToKebabCase("camelCase")      // "camel-case"
strings.ToKebabCase("PascalCase")     // "pascal-case"
strings.ToKebabCase("snake_case")     // "snake-case"
strings.ToKebabCase("HTTPServer")     // "http-server"
strings.ToKebabCase("Title Case")     // "title-case"
strings.ToKebabCase("already-kebab")  // "already-kebab"
```

### ToScreamingSnakeCase

Converts any common case format to SCREAMING_SNAKE_CASE (also known as CONSTANT_CASE).

**Signature:**
```go
func ToScreamingSnakeCase(s string) string
```

**Behavior:**
- First converts to snake_case
- Then converts entire result to uppercase
- Commonly used for constants and environment variables

**Example:**
```go
strings.ToScreamingSnakeCase("camelCase")     // "CAMEL_CASE"
strings.ToScreamingSnakeCase("PascalCase")    // "PASCAL_CASE"
strings.ToScreamingSnakeCase("kebab-case")    // "KEBAB_CASE"
strings.ToScreamingSnakeCase("HTTPServer")    // "HTTP_SERVER"
strings.ToScreamingSnakeCase("maxRetryCount") // "MAX_RETRY_COUNT"
```

### ToTitleCase

Converts any common case format to Title Case (space-separated words with each word capitalized).

**Signature:**
```go
func ToTitleCase(s string) string
```

**Behavior:**
- Inserts spaces before uppercase letters at word boundaries
- Capitalizes the first letter of each word
- Lowercases subsequent letters within each word
- Replaces underscores and hyphens with spaces

**Example:**
```go
strings.ToTitleCase("snake_case")    // "Snake Case"
strings.ToTitleCase("kebab-case")    // "Kebab Case"
strings.ToTitleCase("camelCase")     // "Camel Case"
strings.ToTitleCase("PascalCase")    // "Pascal Case"
strings.ToTitleCase("HTTPServer")    // "Http Server"
strings.ToTitleCase("Already Title") // "Already Title"
```

## Common Use Cases

### API Field Name Conversion

Convert between JSON (camelCase) and database (snake_case) field names:

```go
// JSON to database
jsonField := "firstName"
dbColumn := strings.ToSnakeCase(jsonField) // "first_name"

// Database to JSON
dbColumn := "created_at"
jsonField := strings.ToCamelCase(dbColumn) // "createdAt"
```

### Go Exported Names

Convert configuration keys to Go exported identifiers:

```go
configKey := "max_retry_count"
goName := strings.ToPascalCase(configKey) // "MaxRetryCount"
```

### URL Slugs

Create URL-friendly slugs from titles:

```go
title := "My Blog Post Title"
slug := strings.ToKebabCase(title) // "my-blog-post-title"
```

### Constants Generation

Generate constant names from configuration:

```go
setting := "defaultTimeout"
constName := strings.ToScreamingSnakeCase(setting) // "DEFAULT_TIMEOUT"
```

### Display Labels

Create human-readable labels from code identifiers:

```go
fieldName := "user_email_address"
label := strings.ToTitleCase(fieldName) // "User Email Address"
```

### Round-Trip Conversions

Convert between formats and back:

```go
original := "simple_snake_case"

// snake_case -> camelCase -> snake_case
camel := strings.ToCamelCase(original)     // "simpleSnakeCase"
backToSnake := strings.ToSnakeCase(camel)  // "simple_snake_case"

// snake_case -> kebab-case -> snake_case
kebab := strings.ToKebabCase(original)     // "simple-snake-case"
backToSnake := strings.ToSnakeCase(kebab)  // "simple_snake_case"
```

## Edge Cases

### Empty Strings

All functions return an empty string when given an empty string:

```go
strings.ToSnakeCase("")  // ""
strings.ToCamelCase("")  // ""
strings.ToPascalCase("") // ""
```

### Single Characters

```go
strings.ToSnakeCase("a")  // "a"
strings.ToSnakeCase("A")  // "a"
strings.ToPascalCase("a") // "A"
```

### Numbers

Numbers are passed through unchanged:

```go
strings.ToSnakeCase("user123")     // "user123"
strings.ToSnakeCase("user123Name") // "user123_name"
strings.ToCamelCase("user_123")    // "user123"
```

### Consecutive Uppercase (Acronyms)

Acronyms are handled as single words when followed by lowercase:

```go
strings.ToSnakeCase("HTTPServer")  // "http_server"
strings.ToSnakeCase("getURLPath")  // "get_url_path"
strings.ToKebabCase("XMLParser")   // "xml-parser"
```

### Unicode Characters

Unicode letters are properly handled:

```go
strings.ToSnakeCase("héllo_wörld") // "héllo_wörld"
strings.ToSnakeCase("Héllo_Wörld") // "héllo_wörld"
strings.ToSnakeCase("überCamel")   // "über_camel"
strings.ToSnakeCase("日本Test")    // "日本_test"
```

### Leading/Trailing Delimiters

```go
strings.ToSnakeCase("_test")   // "_test"
strings.ToSnakeCase("test_")   // "test_"
strings.ToCamelCase("_test")   // "test" (leading delimiters are skipped)
strings.ToPascalCase("_test")  // "Test"
strings.ToTitleCase("test_")   // "Test" (trailing spaces are trimmed)
```

## Limitations

### Consecutive Acronyms

Consecutive acronyms are treated as a single word since they are ambiguous without semantic knowledge:

```go
strings.ToSnakeCase("XMLHTTPRequest")  // "xmlhttp_request" (not "xml_http_request")
strings.ToSnakeCase("getHTTPSURL")     // "get_httpsurl" (not "get_https_url")
strings.ToKebabCase("URLAPIKey")       // "urlapi-key" (not "url-api-key")
```

Single acronyms followed by a word are handled correctly:

```go
strings.ToSnakeCase("HTTPServer")      // "http_server"
strings.ToSnakeCase("getURLPath")      // "get_url_path"
```

### Non-Letter Unicode Characters

Characters that are neither uppercase nor lowercase (like Chinese/Japanese characters, emojis) are passed through but don't trigger word boundary detection:

```go
strings.ToSnakeCase("中文HTTPServer")  // "中文http_server" (no underscore after 中文)
strings.ToSnakeCase("test🎉Case")      // "test🎉_case"
```

## Notes

- All functions use O(n) time complexity where n is the number of runes
- Memory allocation uses `strings.Builder` with pre-allocated capacity
- The functions are stateless and safe for concurrent use from multiple goroutines
- Word boundaries are detected based on:
  - Case transitions (lowercase to uppercase)
  - Delimiters (underscores, hyphens, spaces)
  - Acronym detection (consecutive uppercase followed by lowercase)
