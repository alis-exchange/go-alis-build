package strings

import (
	stdstrings "strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/suite"
)

// TestStringsTestSuite runs the testify suite.
func TestStringsTestSuite(t *testing.T) {
	suite.Run(t, new(StringsTestSuite))
}

// StringsTestSuite is a testify suite for comprehensive string conversion tests.
type StringsTestSuite struct {
	suite.Suite
}

// SetupTest runs before each test in the suite.
func (s *StringsTestSuite) SetupTest() {
	// No setup required for stateless functions
}

// TestSuite_SnakeCaseConversions tests ToSnakeCase with various inputs.
func (s *StringsTestSuite) TestSuite_SnakeCaseConversions() {
	// Basic conversions
	s.Equal("camel_case", ToSnakeCase("camelCase"), "camelCase should convert to snake_case")
	s.Equal("pascal_case", ToSnakeCase("PascalCase"), "PascalCase should convert to snake_case")
	s.Equal("kebab_case", ToSnakeCase("kebab-case"), "kebab-case should convert to snake_case")
	s.Equal("http_server", ToSnakeCase("HTTPServer"), "HTTPServer should handle acronyms")
	s.Equal("get_http_response", ToSnakeCase("getHTTPResponse"), "Mixed acronyms should work")

	// Edge cases
	s.Equal("", ToSnakeCase(""), "Empty string should return empty")
	s.Equal("a", ToSnakeCase("a"), "Single char should return lowercase")
	s.Equal("a", ToSnakeCase("A"), "Single uppercase should return lowercase")
}

// TestSuite_CamelCaseConversions tests ToCamelCase with various inputs.
func (s *StringsTestSuite) TestSuite_CamelCaseConversions() {
	// Basic conversions
	s.Equal("snakeCase", ToCamelCase("snake_case"), "snake_case should convert to camelCase")
	s.Equal("kebabCase", ToCamelCase("kebab-case"), "kebab-case should convert to camelCase")
	s.Equal("pascalCase", ToCamelCase("PascalCase"), "PascalCase should lowercase first char")
	s.Equal("spaceSeparated", ToCamelCase("space separated"), "space separated should work")

	// Edge cases
	s.Equal("", ToCamelCase(""), "Empty string should return empty")
	// Regression test for Bug #3: leading delimiter should not capitalize first char
	s.Equal("leading", ToCamelCase("_leading"), "Leading delimiter should be skipped")
	s.Equal("trailing", ToCamelCase("trailing_"), "Trailing delimiter should be handled")
}

// TestSuite_PascalCaseConversions tests ToPascalCase with various inputs.
func (s *StringsTestSuite) TestSuite_PascalCaseConversions() {
	// Basic conversions
	s.Equal("SnakeCase", ToPascalCase("snake_case"), "snake_case should convert to PascalCase")
	s.Equal("KebabCase", ToPascalCase("kebab-case"), "kebab-case should convert to PascalCase")
	s.Equal("CamelCase", ToPascalCase("camelCase"), "camelCase should capitalize first char")
	s.Equal("HttpServer", ToPascalCase("http_server"), "http_server should become HttpServer")

	// Edge cases
	s.Equal("", ToPascalCase(""), "Empty string should return empty")
	s.Equal("A", ToPascalCase("a"), "Single char should be uppercased")
	s.Equal("Leading", ToPascalCase("_leading"), "Leading delimiter should be skipped")
}

// TestSuite_KebabCaseConversions tests ToKebabCase with various inputs.
func (s *StringsTestSuite) TestSuite_KebabCaseConversions() {
	// Basic conversions
	s.Equal("camel-case", ToKebabCase("camelCase"), "camelCase should convert to kebab-case")
	s.Equal("pascal-case", ToKebabCase("PascalCase"), "PascalCase should convert to kebab-case")
	s.Equal("snake-case", ToKebabCase("snake_case"), "snake_case should convert to kebab-case")
	s.Equal("http-server", ToKebabCase("HTTPServer"), "HTTPServer should handle acronyms")

	// Edge cases
	s.Equal("", ToKebabCase(""), "Empty string should return empty")
	s.Equal("a", ToKebabCase("a"), "Single char should return lowercase")
}

// TestSuite_ScreamingSnakeCaseConversions tests ToScreamingSnakeCase with various inputs.
func (s *StringsTestSuite) TestSuite_ScreamingSnakeCaseConversions() {
	// Basic conversions
	s.Equal("CAMEL_CASE", ToScreamingSnakeCase("camelCase"), "camelCase should become SCREAMING")
	s.Equal("PASCAL_CASE", ToScreamingSnakeCase("PascalCase"), "PascalCase should become SCREAMING")
	s.Equal("KEBAB_CASE", ToScreamingSnakeCase("kebab-case"), "kebab-case should become SCREAMING")
	s.Equal("HTTP_SERVER", ToScreamingSnakeCase("HTTPServer"), "Acronyms should be preserved")

	// Edge cases
	s.Equal("", ToScreamingSnakeCase(""), "Empty string should return empty")
	s.Equal("A", ToScreamingSnakeCase("a"), "Single char should be uppercased")
}

// TestSuite_TitleCaseConversions tests ToTitleCase with various inputs.
func (s *StringsTestSuite) TestSuite_TitleCaseConversions() {
	// Basic conversions
	s.Equal("Snake Case", ToTitleCase("snake_case"), "snake_case should become Title Case")
	s.Equal("Kebab Case", ToTitleCase("kebab-case"), "kebab-case should become Title Case")
	s.Equal("Camel Case", ToTitleCase("camelCase"), "camelCase should become Title Case")
	s.Equal("Http Server", ToTitleCase("HTTPServer"), "HTTPServer should become Http Server")

	// Edge cases
	s.Equal("", ToTitleCase(""), "Empty string should return empty")
	s.Equal("A", ToTitleCase("a"), "Single char should be uppercased")
}

// TestSuite_RoundTrips tests that certain conversions can be reversed.
func (s *StringsTestSuite) TestSuite_RoundTrips() {
	// snake_case -> camelCase -> snake_case (for simple cases)
	original := "simple_snake_case"
	camel := ToCamelCase(original)
	backToSnake := ToSnakeCase(camel)
	s.Equal(original, backToSnake, "snake -> camel -> snake should round-trip")

	// snake_case -> PascalCase -> snake_case
	pascal := ToPascalCase(original)
	backToSnake2 := ToSnakeCase(pascal)
	s.Equal(original, backToSnake2, "snake -> pascal -> snake should round-trip")

	// snake_case -> kebab-case -> snake_case
	kebab := ToKebabCase(original)
	s.Equal("simple-snake-case", kebab)
	backToSnake3 := ToSnakeCase(kebab)
	s.Equal(original, backToSnake3, "snake -> kebab -> snake should round-trip")
}

// TestSuite_AssertNotPanics tests that functions don't panic on unusual inputs.
func (s *StringsTestSuite) TestSuite_AssertNotPanics() {
	unusualInputs := []string{
		"", "   ", "___", "---", "\t\n\r",
		"🎉🎊🎁", "日本語テスト", "مرحبا",
		string([]byte{0x00, 0x01, 0x02}), // Control characters
	}

	for _, input := range unusualInputs {
		s.NotPanics(func() { ToSnakeCase(input) }, "ToSnakeCase should not panic on %q", input)
		s.NotPanics(func() { ToCamelCase(input) }, "ToCamelCase should not panic on %q", input)
		s.NotPanics(func() { ToPascalCase(input) }, "ToPascalCase should not panic on %q", input)
		s.NotPanics(func() { ToKebabCase(input) }, "ToKebabCase should not panic on %q", input)
		s.NotPanics(func() { ToScreamingSnakeCase(input) }, "ToScreamingSnakeCase should not panic on %q", input)
		s.NotPanics(func() { ToTitleCase(input) }, "ToTitleCase should not panic on %q", input)
	}
}

// TestSuite_ConsistencyAcrossFunctions tests that conversions are internally consistent.
func (s *StringsTestSuite) TestSuite_ConsistencyAcrossFunctions() {
	// ToScreamingSnakeCase should equal ToUpper(ToSnakeCase)
	inputs := []string{"camelCase", "PascalCase", "kebab-case", "HTTPServer"}
	for _, input := range inputs {
		snake := ToSnakeCase(input)
		screaming := ToScreamingSnakeCase(input)
		s.Equal(screaming, stdstrings.ToUpper(snake),
			"ToScreamingSnakeCase should equal ToUpper(ToSnakeCase) for %q", input)
	}
}

// TestToSnakeCase tests the ToSnakeCase function with various input formats.
func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard conversions from different formats
		{"camelCase", "camelCase", "camel_case"},
		{"PascalCase", "PascalCase", "pascal_case"},
		{"kebab-case", "kebab-case", "kebab_case"},
		{"snake_case", "snake_case", "snake_case"},
		{"space separated", "space separated", "space_separated"},
		{"Title Case", "Title Case", "title_case"},
		{"SCREAMING_SNAKE", "SCREAMING_SNAKE", "screaming_snake"},

		// Acronyms and consecutive uppercase
		{"HTTPServer", "HTTPServer", "http_server"},
		{"getHTTPResponse", "getHTTPResponse", "get_http_response"},
		{"XMLParser", "XMLParser", "xml_parser"},
		{"parseXMLFile", "parseXMLFile", "parse_xml_file"},
		{"simpleXML", "simpleXML", "simple_xml"},
		{"APIEndpoint", "APIEndpoint", "api_endpoint"},
		{"userID", "userID", "user_id"},
		{"getURLPath", "getURLPath", "get_url_path"},

		// Mixed formats
		{"snake_kebab-case", "snake_kebab-case", "snake_kebab_case"},
		{"mixed_Case", "mixed_Case", "mixed_case"},
		{"ALLCAPS", "ALLCAPS", "allcaps"},

		// Numbers
		{"user123", "user123", "user123"},
		{"123test", "123test", "123test"},
		{"test123test", "test123test", "test123test"},
		{"user123Name", "user123Name", "user123_name"},
		{"Version2Update", "Version2Update", "version2_update"},

		// Already snake_case
		{"already_snake_case", "already_snake_case", "already_snake_case"},
		{"http_server", "http_server", "http_server"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestToCamelCase tests the ToCamelCase function with various input formats.
func TestToCamelCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard conversions from different formats
		{"snake_case", "snake_case", "snakeCase"},
		{"kebab-case", "kebab-case", "kebabCase"},
		{"PascalCase", "PascalCase", "pascalCase"},
		{"space separated", "space separated", "spaceSeparated"},
		{"Title Case", "Title Case", "titleCase"},
		{"SCREAMING_SNAKE", "SCREAMING_SNAKE", "sCREAMINGSNAKE"},

		// Already camelCase
		{"alreadyCamel", "alreadyCamel", "alreadyCamel"},
		{"camelCase", "camelCase", "camelCase"},

		// Acronyms and uppercase
		{"http_server", "http_server", "httpServer"},
		{"HTTP_SERVER", "HTTP_SERVER", "hTTPSERVER"},
		{"get_http_response", "get_http_response", "getHttpResponse"},

		// Numbers
		{"user_123", "user_123", "user123"},
		{"user_id_123", "user_id_123", "userId123"},

		// Leading/trailing delimiters - Regression test for Bug #3: leading delimiters are skipped
		{"_leading", "_leading", "leading"},
		{"trailing_", "trailing_", "trailing"},
		{"__double", "__double", "double"},
		{"-leading-hyphen", "-leading-hyphen", "leadingHyphen"},

		// Mixed delimiters
		{"snake_kebab-case", "snake_kebab-case", "snakeKebabCase"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToCamelCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToCamelCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestToPascalCase tests the ToPascalCase function with various input formats.
func TestToPascalCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard conversions from different formats
		{"snake_case", "snake_case", "SnakeCase"},
		{"kebab-case", "kebab-case", "KebabCase"},
		{"camelCase", "camelCase", "CamelCase"},
		{"space separated", "space separated", "SpaceSeparated"},
		{"title case", "title case", "TitleCase"},
		{"SCREAMING_SNAKE", "SCREAMING_SNAKE", "SCREAMINGSNAKE"},

		// Already PascalCase
		{"AlreadyPascal", "AlreadyPascal", "AlreadyPascal"},
		{"PascalCase", "PascalCase", "PascalCase"},

		// Common patterns
		{"http_server", "http_server", "HttpServer"},
		{"get_user_by_id", "get_user_by_id", "GetUserById"},
		{"create_new_user", "create_new_user", "CreateNewUser"},

		// Numbers
		{"user_123", "user_123", "User123"},
		{"version_2_update", "version_2_update", "Version2Update"},

		// Leading/trailing delimiters
		{"_leading", "_leading", "Leading"},
		{"trailing_", "trailing_", "Trailing"},
		{"__double__", "__double__", "Double"},

		// Mixed delimiters
		{"snake_kebab-space case", "snake_kebab-space case", "SnakeKebabSpaceCase"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestToKebabCase tests the ToKebabCase function with various input formats.
func TestToKebabCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard conversions from different formats
		{"camelCase", "camelCase", "camel-case"},
		{"PascalCase", "PascalCase", "pascal-case"},
		{"snake_case", "snake_case", "snake-case"},
		{"space separated", "space separated", "space-separated"},
		{"Title Case", "Title Case", "title-case"},
		{"SCREAMING_SNAKE", "SCREAMING_SNAKE", "screaming-snake"},

		// Already kebab-case
		{"already-kebab", "already-kebab", "already-kebab"},
		{"kebab-case", "kebab-case", "kebab-case"},

		// Acronyms and consecutive uppercase
		{"HTTPServer", "HTTPServer", "http-server"},
		{"XMLHttpRequest", "XMLHttpRequest", "xml-http-request"},
		{"getURLPath", "getURLPath", "get-url-path"},
		{"parseJSON", "parseJSON", "parse-json"},

		// Numbers
		{"user123", "user123", "user123"},
		{"user123Name", "user123Name", "user123-name"},
		{"version2Update", "version2Update", "version2-update"},

		// Mixed delimiters
		{"snake_kebab-case", "snake_kebab-case", "snake-kebab-case"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToKebabCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToKebabCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestToScreamingSnakeCase tests the ToScreamingSnakeCase function with various input formats.
func TestToScreamingSnakeCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard conversions from different formats
		{"camelCase", "camelCase", "CAMEL_CASE"},
		{"PascalCase", "PascalCase", "PASCAL_CASE"},
		{"kebab-case", "kebab-case", "KEBAB_CASE"},
		{"snake_case", "snake_case", "SNAKE_CASE"},
		{"space separated", "space separated", "SPACE_SEPARATED"},
		{"Title Case", "Title Case", "TITLE_CASE"},

		// Already SCREAMING_SNAKE_CASE
		{"ALREADY_SCREAMING", "ALREADY_SCREAMING", "ALREADY_SCREAMING"},
		{"SCREAMING_SNAKE_CASE", "SCREAMING_SNAKE_CASE", "SCREAMING_SNAKE_CASE"},

		// Acronyms
		{"HTTPServer", "HTTPServer", "HTTP_SERVER"},
		{"getHTTPResponse", "getHTTPResponse", "GET_HTTP_RESPONSE"},
		{"maxRetryCount", "maxRetryCount", "MAX_RETRY_COUNT"},

		// Numbers
		{"user123", "user123", "USER123"},
		{"version2Update", "version2Update", "VERSION2_UPDATE"},

		// Common constant patterns
		{"defaultTimeout", "defaultTimeout", "DEFAULT_TIMEOUT"},
		{"maxBufferSize", "maxBufferSize", "MAX_BUFFER_SIZE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToScreamingSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToScreamingSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestToTitleCase tests the ToTitleCase function with various input formats.
func TestToTitleCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard conversions from different formats
		{"snake_case", "snake_case", "Snake Case"},
		{"kebab-case", "kebab-case", "Kebab Case"},
		{"camelCase", "camelCase", "Camel Case"},
		{"PascalCase", "PascalCase", "Pascal Case"},
		{"SCREAMING_SNAKE", "SCREAMING_SNAKE", "Screaming Snake"},

		// Already Title Case
		{"Already Title", "Already Title", "Already Title"},
		{"Title Case", "Title Case", "Title Case"},

		// Acronyms (lowercased after first letter of each word)
		{"HTTPServer", "HTTPServer", "Http Server"},
		{"userID", "userID", "User Id"},
		{"parseXMLFile", "parseXMLFile", "Parse Xml File"},

		// Numbers
		{"user123", "user123", "User123"},
		{"version2Update", "version2Update", "Version2 Update"},

		// Real-world examples
		{"firstName", "firstName", "First Name"},
		{"lastName", "lastName", "Last Name"},
		{"emailAddress", "emailAddress", "Email Address"},
		{"phoneNumber", "phoneNumber", "Phone Number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToTitleCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToTitleCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestEmptyStrings tests all functions with empty string input.
func TestEmptyStrings(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) string
	}{
		{"ToSnakeCase", ToSnakeCase},
		{"ToCamelCase", ToCamelCase},
		{"ToPascalCase", ToPascalCase},
		{"ToKebabCase", ToKebabCase},
		{"ToScreamingSnakeCase", ToScreamingSnakeCase},
		{"ToTitleCase", ToTitleCase},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn("")
			if result != "" {
				t.Errorf("%s(\"\") = %q, want \"\"", tt.name, result)
			}
		})
	}
}

// TestSingleCharacters tests all functions with single character inputs.
func TestSingleCharacters(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string) string
		input    string
		expected string
	}{
		// Lowercase single char
		{"ToSnakeCase lowercase", ToSnakeCase, "a", "a"},
		{"ToCamelCase lowercase", ToCamelCase, "a", "a"},
		{"ToPascalCase lowercase", ToPascalCase, "a", "A"},
		{"ToKebabCase lowercase", ToKebabCase, "a", "a"},
		{"ToScreamingSnakeCase lowercase", ToScreamingSnakeCase, "a", "A"},
		{"ToTitleCase lowercase", ToTitleCase, "a", "A"},

		// Uppercase single char
		{"ToSnakeCase uppercase", ToSnakeCase, "A", "a"},
		{"ToCamelCase uppercase", ToCamelCase, "A", "a"},
		{"ToPascalCase uppercase", ToPascalCase, "A", "A"},
		{"ToKebabCase uppercase", ToKebabCase, "A", "a"},
		{"ToScreamingSnakeCase uppercase", ToScreamingSnakeCase, "A", "A"},
		{"ToTitleCase uppercase", ToTitleCase, "A", "A"},

		// Single delimiter
		{"ToSnakeCase underscore", ToSnakeCase, "_", "_"},
		{"ToCamelCase underscore", ToCamelCase, "_", ""},
		{"ToPascalCase underscore", ToPascalCase, "_", ""},
		{"ToKebabCase underscore", ToKebabCase, "_", "-"},
		{"ToScreamingSnakeCase underscore", ToScreamingSnakeCase, "_", "_"},
		{"ToTitleCase underscore", ToTitleCase, "_", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			if result != tt.expected {
				t.Errorf("%s(%q) = %q, want %q", tt.name, tt.input, result, tt.expected)
			}
		})
	}
}

// TestConsecutiveUppercase tests handling of acronyms and consecutive uppercase letters.
func TestConsecutiveUppercase(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		snake     string
		camel     string
		pascal    string
		kebab     string
		screaming string
		title     string
	}{
		{
			name:      "HTTP",
			input:     "HTTP",
			snake:     "http",
			camel:     "hTTP",
			pascal:    "HTTP",
			kebab:     "http",
			screaming: "HTTP",
			title:     "Http",
		},
		{
			name:      "API",
			input:     "API",
			snake:     "api",
			camel:     "aPI",
			pascal:    "API",
			kebab:     "api",
			screaming: "API",
			title:     "Api",
		},
		{
			name:      "HTTPServer",
			input:     "HTTPServer",
			snake:     "http_server",
			camel:     "hTTPServer",
			pascal:    "HTTPServer",
			kebab:     "http-server",
			screaming: "HTTP_SERVER",
			title:     "Http Server",
		},
		{
			name:      "getHTTPSURL",
			input:     "getHTTPSURL",
			snake:     "get_httpsurl",
			camel:     "getHTTPSURL",
			pascal:    "GetHTTPSURL",
			kebab:     "get-httpsurl",
			screaming: "GET_HTTPSURL",
			title:     "Get Httpsurl",
		},
		{
			name:      "XMLHTTPRequest",
			input:     "XMLHTTPRequest",
			snake:     "xmlhttp_request",
			camel:     "xMLHTTPRequest",
			pascal:    "XMLHTTPRequest",
			kebab:     "xmlhttp-request",
			screaming: "XMLHTTP_REQUEST",
			title:     "Xmlhttp Request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_snake", func(t *testing.T) {
			if result := ToSnakeCase(tt.input); result != tt.snake {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, result, tt.snake)
			}
		})
		t.Run(tt.name+"_camel", func(t *testing.T) {
			if result := ToCamelCase(tt.input); result != tt.camel {
				t.Errorf("ToCamelCase(%q) = %q, want %q", tt.input, result, tt.camel)
			}
		})
		t.Run(tt.name+"_pascal", func(t *testing.T) {
			if result := ToPascalCase(tt.input); result != tt.pascal {
				t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, result, tt.pascal)
			}
		})
		t.Run(tt.name+"_kebab", func(t *testing.T) {
			if result := ToKebabCase(tt.input); result != tt.kebab {
				t.Errorf("ToKebabCase(%q) = %q, want %q", tt.input, result, tt.kebab)
			}
		})
		t.Run(tt.name+"_screaming", func(t *testing.T) {
			if result := ToScreamingSnakeCase(tt.input); result != tt.screaming {
				t.Errorf("ToScreamingSnakeCase(%q) = %q, want %q", tt.input, result, tt.screaming)
			}
		})
		t.Run(tt.name+"_title", func(t *testing.T) {
			if result := ToTitleCase(tt.input); result != tt.title {
				t.Errorf("ToTitleCase(%q) = %q, want %q", tt.input, result, tt.title)
			}
		})
	}
}

// TestNumericStrings tests handling of strings containing numbers.
func TestNumericStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		snake string
		kebab string
	}{
		{"leading numbers", "123test", "123test", "123test"},
		{"trailing numbers", "test123", "test123", "test123"},
		{"middle numbers", "test123test", "test123test", "test123test"},
		{"numbers with case change", "user123Name", "user123_name", "user123-name"},
		{"version number", "Version2", "version2", "version2"},
		{"multiple number groups", "test1and2and3", "test1and2and3", "test1and2and3"},
		{"only numbers", "12345", "12345", "12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_snake", func(t *testing.T) {
			if result := ToSnakeCase(tt.input); result != tt.snake {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, result, tt.snake)
			}
		})
		t.Run(tt.name+"_kebab", func(t *testing.T) {
			if result := ToKebabCase(tt.input); result != tt.kebab {
				t.Errorf("ToKebabCase(%q) = %q, want %q", tt.input, result, tt.kebab)
			}
		})
	}
}

// TestUnicodeStrings tests handling of Unicode characters.
func TestUnicodeStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		snake string
	}{
		{"accented lowercase", "héllo_wörld", "héllo_wörld"},
		{"accented mixed", "Héllo_Wörld", "héllo_wörld"},
		{"german eszett", "straße", "straße"},
		{"greek letters", "αβγδ", "αβγδ"},
		{"mixed ascii unicode", "testÜber", "test_über"},
		{"emoji", "test🎉case", "test🎉case"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToSnakeCase(tt.input)
			if result != tt.snake {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, result, tt.snake)
			}
			// Verify result is valid UTF-8
			if !utf8.ValidString(result) {
				t.Errorf("ToSnakeCase(%q) produced invalid UTF-8: %q", tt.input, result)
			}
		})
	}
}

// TestUnicodeMultiByte tests proper handling of multi-byte Unicode characters
// followed by uppercase letters.
// Regression test for Bug #1: multi-byte Unicode indexing bug.
func TestUnicodeMultiByte(t *testing.T) {
	t.Run("ToSnakeCase", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{"japanese then upper", "日本Test", "日本_test"},
			{"japanese then camel", "日本TestCase", "日本_test_case"},
			{"umlaut camel", "überCamel", "über_camel"},
			{"cafe api", "caféAPI", "café_api"},
			{"emoji then upper", "test🎉Case", "test🎉_case"},
			{"cyrillic mixed", "МоскваTest", "москва_test"},
			{"chinese http", "中文HTTPServer", "中文http_server"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := ToSnakeCase(tt.input)
				if result != tt.expected {
					t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
				}
			})
		}
	})

	t.Run("ToKebabCase", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{"japanese then upper", "日本Test", "日本-test"},
			{"umlaut camel", "überCamel", "über-camel"},
			{"emoji then upper", "test🎉Case", "test🎉-case"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := ToKebabCase(tt.input)
				if result != tt.expected {
					t.Errorf("ToKebabCase(%q) = %q, want %q", tt.input, result, tt.expected)
				}
			})
		}
	})

	t.Run("ToTitleCase", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{"japanese then upper", "日本Test", "日本 Test"},
			{"umlaut camel", "überCamel", "Über Camel"},
			{"emoji then upper", "test🎉Case", "Test🎉 Case"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := ToTitleCase(tt.input)
				if result != tt.expected {
					t.Errorf("ToTitleCase(%q) = %q, want %q", tt.input, result, tt.expected)
				}
			})
		}
	})
}

// TestConsecutiveDelimiters tests handling of multiple consecutive delimiters.
func TestConsecutiveDelimiters(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		snake  string
		camel  string
		pascal string
	}{
		{"double underscore", "a__b", "a__b", "aB", "AB"},
		{"double hyphen", "a--b", "a__b", "aB", "AB"},
		{"double space", "a  b", "a__b", "aB", "AB"},
		{"triple underscore", "a___b", "a___b", "aB", "AB"},
		{"mixed consecutive", "a_-_b", "a___b", "aB", "AB"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_snake", func(t *testing.T) {
			if result := ToSnakeCase(tt.input); result != tt.snake {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, result, tt.snake)
			}
		})
		t.Run(tt.name+"_camel", func(t *testing.T) {
			if result := ToCamelCase(tt.input); result != tt.camel {
				t.Errorf("ToCamelCase(%q) = %q, want %q", tt.input, result, tt.camel)
			}
		})
		t.Run(tt.name+"_pascal", func(t *testing.T) {
			if result := ToPascalCase(tt.input); result != tt.pascal {
				t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, result, tt.pascal)
			}
		})
	}
}

// TestLeadingTrailingDelimiters tests handling of delimiters at string boundaries.
func TestLeadingTrailingDelimiters(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		snake  string
		camel  string
		pascal string
	}{
		{"leading underscore", "_test", "_test", "test", "Test"},
		{"trailing underscore", "test_", "test_", "test", "Test"},
		{"both underscore", "_test_", "_test_", "test", "Test"},
		{"leading hyphen", "-test", "_test", "test", "Test"},
		{"trailing hyphen", "test-", "test_", "test", "Test"},
		{"leading space", " test", "_test", "test", "Test"},
		{"trailing space", "test ", "test_", "test", "Test"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_snake", func(t *testing.T) {
			if result := ToSnakeCase(tt.input); result != tt.snake {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, result, tt.snake)
			}
		})
		t.Run(tt.name+"_camel", func(t *testing.T) {
			if result := ToCamelCase(tt.input); result != tt.camel {
				t.Errorf("ToCamelCase(%q) = %q, want %q", tt.input, result, tt.camel)
			}
		})
		t.Run(tt.name+"_pascal", func(t *testing.T) {
			if result := ToPascalCase(tt.input); result != tt.pascal {
				t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, result, tt.pascal)
			}
		})
	}
}

// TestToTitleCase_TrailingDelimiters verifies trailing delimiters don't produce trailing spaces.
// Regression test for Bug #4.
func TestToTitleCase_TrailingDelimiters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"trailing underscore", "test_", "Test"},
		{"trailing hyphen", "test-", "Test"},
		{"multiple trailing", "test___", "Test"},
		{"only underscores", "___", ""},
		{"only hyphens", "---", ""},
		{"only spaces", "   ", ""},
		{"word with trailing mixed", "hello_-_", "Hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToTitleCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToTitleCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
			// Verify no trailing whitespace
			if result != stdstrings.TrimRight(result, " ") {
				t.Errorf("ToTitleCase(%q) has trailing whitespace: %q", tt.input, result)
			}
		})
	}
}

// TestDelimiterOnlyInputs tests behavior when input contains only delimiters.
// Regression test for Bug #5.
func TestDelimiterOnlyInputs(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		snake     string
		camel     string
		pascal    string
		kebab     string
		screaming string
		title     string
	}{
		{"triple underscore", "___", "___", "", "", "---", "___", ""},
		{"triple hyphen", "---", "___", "", "", "---", "___", ""},
		{"triple space", "   ", "___", "", "", "---", "___", ""},
		{"mixed delimiters", "_-_ ", "____", "", "", "----", "____", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_snake", func(t *testing.T) {
			if result := ToSnakeCase(tt.input); result != tt.snake {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, result, tt.snake)
			}
		})
		t.Run(tt.name+"_camel", func(t *testing.T) {
			if result := ToCamelCase(tt.input); result != tt.camel {
				t.Errorf("ToCamelCase(%q) = %q, want %q", tt.input, result, tt.camel)
			}
		})
		t.Run(tt.name+"_pascal", func(t *testing.T) {
			if result := ToPascalCase(tt.input); result != tt.pascal {
				t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, result, tt.pascal)
			}
		})
		t.Run(tt.name+"_kebab", func(t *testing.T) {
			if result := ToKebabCase(tt.input); result != tt.kebab {
				t.Errorf("ToKebabCase(%q) = %q, want %q", tt.input, result, tt.kebab)
			}
		})
		t.Run(tt.name+"_screaming", func(t *testing.T) {
			if result := ToScreamingSnakeCase(tt.input); result != tt.screaming {
				t.Errorf("ToScreamingSnakeCase(%q) = %q, want %q", tt.input, result, tt.screaming)
			}
		})
		t.Run(tt.name+"_title", func(t *testing.T) {
			if result := ToTitleCase(tt.input); result != tt.title {
				t.Errorf("ToTitleCase(%q) = %q, want %q", tt.input, result, tt.title)
			}
		})
	}
}

// FuzzToSnakeCase tests ToSnakeCase with random inputs to ensure no panics
// and that the output is idempotent (applying twice yields same result).
func FuzzToSnakeCase(f *testing.F) {
	// Seed corpus with known edge cases
	seeds := []string{
		"", "a", "A", "camelCase", "PascalCase", "snake_case",
		"kebab-case", "HTTPServer", "getHTTPResponse", "XMLParser",
		"user123", "123test", "_leading", "trailing_", "__double__",
		"a__b", "héllo", "αβγδ", "test🎉case",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := ToSnakeCase(input)

		// Idempotence: applying twice should yield same result
		result2 := ToSnakeCase(result)
		if result != result2 {
			t.Errorf("not idempotent: ToSnakeCase(%q) = %q, ToSnakeCase(%q) = %q",
				input, result, result, result2)
		}

		// Result should be valid UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("ToSnakeCase(%q) produced invalid UTF-8", input)
		}
	})
}

// FuzzToCamelCase tests ToCamelCase with random inputs.
func FuzzToCamelCase(f *testing.F) {
	seeds := []string{
		"", "a", "A", "snake_case", "kebab-case", "PascalCase",
		"space separated", "_leading", "trailing_", "a__b",
		"user123", "héllo_wörld",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := ToCamelCase(input)

		// Idempotence check
		result2 := ToCamelCase(result)
		if result != result2 {
			t.Errorf("not idempotent: ToCamelCase(%q) = %q, ToCamelCase(%q) = %q",
				input, result, result, result2)
		}

		// Result should be valid UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("ToCamelCase(%q) produced invalid UTF-8", input)
		}
	})
}

// FuzzToPascalCase tests ToPascalCase with random inputs.
func FuzzToPascalCase(f *testing.F) {
	seeds := []string{
		"", "a", "A", "snake_case", "kebab-case", "camelCase",
		"space separated", "_leading", "trailing_", "a__b",
		"user123", "héllo_wörld",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := ToPascalCase(input)

		// Idempotence check
		result2 := ToPascalCase(result)
		if result != result2 {
			t.Errorf("not idempotent: ToPascalCase(%q) = %q, ToPascalCase(%q) = %q",
				input, result, result, result2)
		}

		// Result should be valid UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("ToPascalCase(%q) produced invalid UTF-8", input)
		}
	})
}

// FuzzToKebabCase tests ToKebabCase with random inputs.
func FuzzToKebabCase(f *testing.F) {
	seeds := []string{
		"", "a", "A", "camelCase", "PascalCase", "snake_case",
		"already-kebab", "HTTPServer", "_leading", "trailing_",
		"user123", "héllo_wörld",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := ToKebabCase(input)

		// Idempotence check
		result2 := ToKebabCase(result)
		if result != result2 {
			t.Errorf("not idempotent: ToKebabCase(%q) = %q, ToKebabCase(%q) = %q",
				input, result, result, result2)
		}

		// Result should be valid UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("ToKebabCase(%q) produced invalid UTF-8", input)
		}
	})
}

// FuzzToScreamingSnakeCase tests ToScreamingSnakeCase with random inputs.
func FuzzToScreamingSnakeCase(f *testing.F) {
	seeds := []string{
		"", "a", "A", "camelCase", "PascalCase", "snake_case",
		"kebab-case", "ALREADY_SCREAMING", "HTTPServer",
		"user123", "héllo_wörld",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := ToScreamingSnakeCase(input)

		// Idempotence check
		result2 := ToScreamingSnakeCase(result)
		if result != result2 {
			t.Errorf("not idempotent: ToScreamingSnakeCase(%q) = %q, ToScreamingSnakeCase(%q) = %q",
				input, result, result, result2)
		}

		// Result should be valid UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("ToScreamingSnakeCase(%q) produced invalid UTF-8", input)
		}
	})
}

// FuzzToTitleCase tests ToTitleCase with random inputs.
func FuzzToTitleCase(f *testing.F) {
	seeds := []string{
		"", "a", "A", "snake_case", "kebab-case", "camelCase",
		"PascalCase", "Already Title", "HTTPServer",
		"user123", "héllo_wörld",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := ToTitleCase(input)

		// Idempotence check
		result2 := ToTitleCase(result)
		if result != result2 {
			t.Errorf("not idempotent: ToTitleCase(%q) = %q, ToTitleCase(%q) = %q",
				input, result, result, result2)
		}

		// Result should be valid UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("ToTitleCase(%q) produced invalid UTF-8", input)
		}
	})
}

// BenchmarkToSnakeCase benchmarks the ToSnakeCase function.
func BenchmarkToSnakeCase(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{"short_camel", "camelCase"},
		{"long_camel", "thisIsAVeryLongCamelCaseStringForBenchmarking"},
		{"with_acronym", "getHTTPResponseFromAPIServer"},
		{"already_snake", "already_snake_case_string"},
		{"mixed", "mixed_Case-with-Different_Delimiters"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ToSnakeCase(bm.input)
			}
		})
	}
}

// BenchmarkToCamelCase benchmarks the ToCamelCase function.
func BenchmarkToCamelCase(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{"short_snake", "snake_case"},
		{"long_snake", "this_is_a_very_long_snake_case_string_for_benchmarking"},
		{"kebab", "kebab-case-string"},
		{"already_camel", "alreadyCamelCaseString"},
		{"mixed", "mixed_Case-with-Different_Delimiters"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ToCamelCase(bm.input)
			}
		})
	}
}

// BenchmarkToPascalCase benchmarks the ToPascalCase function.
func BenchmarkToPascalCase(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{"short_snake", "snake_case"},
		{"long_snake", "this_is_a_very_long_snake_case_string_for_benchmarking"},
		{"camel", "camelCaseString"},
		{"already_pascal", "AlreadyPascalCaseString"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ToPascalCase(bm.input)
			}
		})
	}
}

// BenchmarkToKebabCase benchmarks the ToKebabCase function.
func BenchmarkToKebabCase(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{"short_camel", "camelCase"},
		{"long_camel", "thisIsAVeryLongCamelCaseStringForBenchmarking"},
		{"with_acronym", "getHTTPResponseFromAPIServer"},
		{"snake", "snake_case_string"},
		{"already_kebab", "already-kebab-case-string"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ToKebabCase(bm.input)
			}
		})
	}
}

// BenchmarkToScreamingSnakeCase benchmarks the ToScreamingSnakeCase function.
func BenchmarkToScreamingSnakeCase(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{"short_camel", "camelCase"},
		{"long_camel", "thisIsAVeryLongCamelCaseStringForBenchmarking"},
		{"with_acronym", "getHTTPResponseFromAPIServer"},
		{"already_screaming", "ALREADY_SCREAMING_SNAKE_CASE"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ToScreamingSnakeCase(bm.input)
			}
		})
	}
}

// BenchmarkToTitleCase benchmarks the ToTitleCase function.
func BenchmarkToTitleCase(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{"short_snake", "snake_case"},
		{"long_snake", "this_is_a_very_long_snake_case_string_for_benchmarking"},
		{"camel", "camelCaseString"},
		{"kebab", "kebab-case-string"},
		{"already_title", "Already Title Case String"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ToTitleCase(bm.input)
			}
		})
	}
}

// BenchmarkAllFunctions provides a comparative benchmark across all functions.
func BenchmarkAllFunctions(b *testing.B) {
	input := "getHTTPResponseFromAPIServer"

	b.Run("ToSnakeCase", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ToSnakeCase(input)
		}
	})

	b.Run("ToCamelCase", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ToCamelCase(input)
		}
	})

	b.Run("ToPascalCase", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ToPascalCase(input)
		}
	})

	b.Run("ToKebabCase", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ToKebabCase(input)
		}
	})

	b.Run("ToScreamingSnakeCase", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ToScreamingSnakeCase(input)
		}
	})

	b.Run("ToTitleCase", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ToTitleCase(input)
		}
	})
}
