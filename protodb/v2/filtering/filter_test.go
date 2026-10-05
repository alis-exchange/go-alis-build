package filtering

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ParserSuite is the main test suite for the filtering parser
type ParserSuite struct {
	suite.Suite
	parser *Parser
}

// SetupSuite runs once before all tests in the suite
func (s *ParserSuite) SetupSuite() {
	// Create parser with common identifiers for testing
	identifiers := []Identifier{
		Timestamp("create_time"),
		Timestamp("update_time"),
		Duration("expire_after"),
		Duration("timeout"),
		Date("effective_date"),
		Date("birth_date"),
		Reserved("select"),
		Reserved("from"),
		EnumString("status", "test.Status"),
		EnumInteger("priority", "test.Priority"),
	}

	parser, err := NewParser(identifiers...)
	require.NoError(s.T(), err, "Failed to create parser")
	s.parser = parser
}

// assertStatement is a helper to validate both SQL and params
func (s *ParserSuite) assertStatement(filter, expectedSQL string, expectedParams map[string]any) {
	stmt, err := s.parser.Parse(filter)
	require.NoError(s.T(), err, "Failed to parse filter: %s", filter)
	assert.Equal(s.T(), expectedSQL, stmt.SQL, "SQL mismatch for filter: %s", filter)
	assert.Equal(s.T(), expectedParams, stmt.Params, "Params mismatch for filter: %s", filter)
}

// assertError is a helper to validate that parsing returns an error
func (s *ParserSuite) assertError(filter string) {
	_, err := s.parser.Parse(filter)
	assert.Error(s.T(), err, "Expected error for filter: %s", filter)
}

// TestParserSuite runs the main parser test suite
func TestParserSuite(t *testing.T) {
	suite.Run(t, new(ParserSuite))
}

// =============================================================================
// Comparison Operators Tests
// =============================================================================

// TestEquality checks that == against a string compiles to = with the string
// bound as @p0.
func (s *ParserSuite) TestEquality() {
	s.assertStatement(
		"name == 'Alice'",
		"name = @p0",
		map[string]any{"p0": "Alice"},
	)
}

// TestEqualityWithInt checks that an integer literal on the right of == is
// bound as an int64 parameter.
func (s *ParserSuite) TestEqualityWithInt() {
	s.assertStatement(
		"age == 25",
		"age = @p0",
		map[string]any{"p0": int64(25)},
	)
}

// TestEqualityWithBool checks that a true literal on the right of == is bound
// as a bool parameter.
func (s *ParserSuite) TestEqualityWithBool() {
	s.assertStatement(
		"active == true",
		"active = @p0",
		map[string]any{"p0": true},
	)
}

// TestInequality checks that != compiles to SQL != with the string bound as
// @p0.
func (s *ParserSuite) TestInequality() {
	s.assertStatement(
		"name != 'Alice'",
		"name != @p0",
		map[string]any{"p0": "Alice"},
	)
}

// TestGreaterThan checks that > compiles to SQL > with the integer bound as an
// int64 parameter.
func (s *ParserSuite) TestGreaterThan() {
	s.assertStatement(
		"age > 18",
		"age > @p0",
		map[string]any{"p0": int64(18)},
	)
}

// TestGreaterThanOrEqual checks that >= compiles to SQL >= with the integer
// bound as an int64 parameter.
func (s *ParserSuite) TestGreaterThanOrEqual() {
	s.assertStatement(
		"age >= 18",
		"age >= @p0",
		map[string]any{"p0": int64(18)},
	)
}

// TestLessThan checks that < compiles to SQL < with the integer bound as an
// int64 parameter.
func (s *ParserSuite) TestLessThan() {
	s.assertStatement(
		"age < 65",
		"age < @p0",
		map[string]any{"p0": int64(65)},
	)
}

// TestLessThanOrEqual checks that <= compiles to SQL <= with the integer bound
// as an int64 parameter.
func (s *ParserSuite) TestLessThanOrEqual() {
	s.assertStatement(
		"age <= 65",
		"age <= @p0",
		map[string]any{"p0": int64(65)},
	)
}

// TestComparisonWithDouble checks that a decimal literal such as 19.99 is bound
// as a float64 parameter.
func (s *ParserSuite) TestComparisonWithDouble() {
	s.assertStatement(
		"price > 19.99",
		"price > @p0",
		map[string]any{"p0": 19.99},
	)
}

// =============================================================================
// Logical Operators Tests
// =============================================================================

// TestLogicalAnd checks that && compiles to a parenthesized AND with parameters
// numbered left to right.
func (s *ParserSuite) TestLogicalAnd() {
	s.assertStatement(
		"a == 1 && b == 2",
		"(a = @p0 AND b = @p1)",
		map[string]any{"p0": int64(1), "p1": int64(2)},
	)
}

// TestLogicalAndKeyword checks that the AND keyword produces the same
// parenthesized AND as &&.
func (s *ParserSuite) TestLogicalAndKeyword() {
	s.assertStatement(
		"a == 1 AND b == 2",
		"(a = @p0 AND b = @p1)",
		map[string]any{"p0": int64(1), "p1": int64(2)},
	)
}

// TestLogicalOr checks that || compiles to a parenthesized OR with parameters
// numbered left to right.
func (s *ParserSuite) TestLogicalOr() {
	s.assertStatement(
		"a == 1 || b == 2",
		"(a = @p0 OR b = @p1)",
		map[string]any{"p0": int64(1), "p1": int64(2)},
	)
}

// TestLogicalOrKeyword checks that the OR keyword produces the same
// parenthesized OR as ||.
func (s *ParserSuite) TestLogicalOrKeyword() {
	s.assertStatement(
		"a == 1 OR b == 2",
		"(a = @p0 OR b = @p1)",
		map[string]any{"p0": int64(1), "p1": int64(2)},
	)
}

// TestNestedLogical checks that a parenthesized AND group inside an OR keeps
// its own parentheses in the SQL.
func (s *ParserSuite) TestNestedLogical() {
	s.assertStatement(
		"(a == 1 && b == 2) || c == 3",
		"((a = @p0 AND b = @p1) OR c = @p2)",
		map[string]any{"p0": int64(1), "p1": int64(2), "p2": int64(3)},
	)
}

// TestComplexLogical checks that a parenthesized OR on the right of && is
// nested inside the outer AND.
func (s *ParserSuite) TestComplexLogical() {
	s.assertStatement(
		"a == 1 && (b == 2 || c == 3)",
		"(a = @p0 AND (b = @p1 OR c = @p2))",
		map[string]any{"p0": int64(1), "p1": int64(2), "p2": int64(3)},
	)
}

// TestMultipleAnd checks that a chain of && is left-associative, so the first
// two terms are grouped together.
func (s *ParserSuite) TestMultipleAnd() {
	s.assertStatement(
		"a == 1 && b == 2 && c == 3",
		"((a = @p0 AND b = @p1) AND c = @p2)",
		map[string]any{"p0": int64(1), "p1": int64(2), "p2": int64(3)},
	)
}

// =============================================================================
// String Functions Tests
// =============================================================================

// TestLike checks that like(name, pattern) compiles to name LIKE @p0 with the
// pattern bound unchanged.
func (s *ParserSuite) TestLike() {
	s.assertStatement(
		"like(name, '%Alice%')",
		"name LIKE @p0",
		map[string]any{"p0": "%Alice%"},
	)
}

// TestLikeStartsWith checks that a trailing-wildcard like pattern is passed
// through as a bound LIKE parameter.
func (s *ParserSuite) TestLikeStartsWith() {
	s.assertStatement(
		"like(name, 'Alice%')",
		"name LIKE @p0",
		map[string]any{"p0": "Alice%"},
	)
}

// TestLikeEndsWith checks that a leading-wildcard like pattern is passed
// through as a bound LIKE parameter.
func (s *ParserSuite) TestLikeEndsWith() {
	s.assertStatement(
		"like(name, '%Alice')",
		"name LIKE @p0",
		map[string]any{"p0": "%Alice"},
	)
}

// TestLower checks that lower(name) compiles to LOWER(name) on the left of a
// bound equality.
func (s *ParserSuite) TestLower() {
	s.assertStatement(
		"lower(name) == 'alice'",
		"LOWER(name) = @p0",
		map[string]any{"p0": "alice"},
	)
}

// TestUpper checks that upper(name) compiles to UPPER(name) on the left of a
// bound equality.
func (s *ParserSuite) TestUpper() {
	s.assertStatement(
		"upper(name) == 'ALICE'",
		"UPPER(name) = @p0",
		map[string]any{"p0": "ALICE"},
	)
}

// TestPrefix checks that prefix(name, s) compiles to STARTS_WITH(name, @p0).
func (s *ParserSuite) TestPrefix() {
	s.assertStatement(
		"prefix(name, 'Al')",
		"STARTS_WITH(name, @p0)",
		map[string]any{"p0": "Al"},
	)
}

// TestSuffix checks that suffix(name, s) compiles to ENDS_WITH(name, @p0).
func (s *ParserSuite) TestSuffix() {
	s.assertStatement(
		"suffix(name, 'ce')",
		"ENDS_WITH(name, @p0)",
		map[string]any{"p0": "ce"},
	)
}

// TestLikeLower checks that like over lower(name) compiles to LOWER(name) LIKE
// @p0.
func (s *ParserSuite) TestLikeLower() {
	s.assertStatement(
		"like(lower(name), '%alice%')",
		"LOWER(name) LIKE @p0",
		map[string]any{"p0": "%alice%"},
	)
}

// TestLikeUpper checks that like over upper(name) compiles to UPPER(name) LIKE
// @p0.
func (s *ParserSuite) TestLikeUpper() {
	s.assertStatement(
		"like(upper(name), '%ALICE%')",
		"UPPER(name) LIKE @p0",
		map[string]any{"p0": "%ALICE%"},
	)
}

// TestPrefixWithLower checks that prefix over lower(name) compiles to
// STARTS_WITH(LOWER(name), @p0).
func (s *ParserSuite) TestPrefixWithLower() {
	s.assertStatement(
		"prefix(lower(name), 'al')",
		"STARTS_WITH(LOWER(name), @p0)",
		map[string]any{"p0": "al"},
	)
}

// TestSuffixWithUpper checks that suffix over upper(name) compiles to
// ENDS_WITH(UPPER(name), @p0).
func (s *ParserSuite) TestSuffixWithUpper() {
	s.assertStatement(
		"suffix(upper(name), 'CE')",
		"ENDS_WITH(UPPER(name), @p0)",
		map[string]any{"p0": "CE"},
	)
}

// =============================================================================
// Multi-Arg Functions Tests
// =============================================================================

// TestConcat checks that concat keeps field arguments as columns and binds the
// string separator as @p0.
func (s *ParserSuite) TestConcat() {
	s.assertStatement(
		"concat(first_name, ' ', last_name)",
		"CONCAT(first_name, @p0, last_name)",
		map[string]any{"p0": " "},
	)
}

// TestConcatTwoFields checks that concat of two fields compiles to
// CONCAT(first_name, last_name) with no parameters.
func (s *ParserSuite) TestConcatTwoFields() {
	s.assertStatement(
		"concat(first_name, last_name)",
		"CONCAT(first_name, last_name)",
		map[string]any{},
	)
}

// TestConcatWithConstant checks that a leading string constant in concat is
// bound as @p0 ahead of the field.
func (s *ParserSuite) TestConcatWithConstant() {
	s.assertStatement(
		"concat('Hello ', name)",
		"CONCAT(@p0, name)",
		map[string]any{"p0": "Hello "},
	)
}

// TestGreatest checks that greatest keeps field arguments as columns and binds
// the integer argument.
func (s *ParserSuite) TestGreatest() {
	s.assertStatement(
		"greatest(a, b, 10)",
		"GREATEST(a, b, @p0)",
		map[string]any{"p0": int64(10)},
	)
}

// TestGreatestTwoFields checks that greatest of two fields compiles to
// GREATEST(price, min_price) with no parameters.
func (s *ParserSuite) TestGreatestTwoFields() {
	s.assertStatement(
		"greatest(price, min_price)",
		"GREATEST(price, min_price)",
		map[string]any{},
	)
}

// TestLeast checks that least keeps field arguments as columns and binds the
// integer argument.
func (s *ParserSuite) TestLeast() {
	s.assertStatement(
		"least(a, b, 10)",
		"LEAST(a, b, @p0)",
		map[string]any{"p0": int64(10)},
	)
}

// TestLeastTwoFields checks that least of two fields compiles to
// LEAST(quantity, max_quantity) with no parameters.
func (s *ParserSuite) TestLeastTwoFields() {
	s.assertStatement(
		"least(quantity, max_quantity)",
		"LEAST(quantity, max_quantity)",
		map[string]any{},
	)
}

// TestCoalesce checks that coalesce keeps field arguments as columns and binds
// the trailing string default.
func (s *ParserSuite) TestCoalesce() {
	s.assertStatement(
		"coalesce(nickname, name, 'Unknown')",
		"COALESCE(nickname, name, @p0)",
		map[string]any{"p0": "Unknown"},
	)
}

// TestCoalesceTwoFields checks that coalesce of two fields compiles to
// COALESCE(nickname, name) with no parameters.
func (s *ParserSuite) TestCoalesceTwoFields() {
	s.assertStatement(
		"coalesce(nickname, name)",
		"COALESCE(nickname, name)",
		map[string]any{},
	)
}

// TestIfnull checks that ifnull(nickname, 'N/A') compiles to IFNULL(nickname,
// @p0) with the default bound.
func (s *ParserSuite) TestIfnull() {
	s.assertStatement(
		"ifnull(nickname, 'N/A')",
		"IFNULL(nickname, @p0)",
		map[string]any{"p0": "N/A"},
	)
}

// TestIfnullWithInt checks that an integer default in ifnull is bound as an
// int64 parameter.
func (s *ParserSuite) TestIfnullWithInt() {
	s.assertStatement(
		"ifnull(count, 0)",
		"IFNULL(count, @p0)",
		map[string]any{"p0": int64(0)},
	)
}

// TestIfnullTwoFields checks that ifnull of two fields compiles to IFNULL with
// both columns and no parameters.
func (s *ParserSuite) TestIfnullTwoFields() {
	s.assertStatement(
		"ifnull(primary_email, secondary_email)",
		"IFNULL(primary_email, secondary_email)",
		map[string]any{},
	)
}

// TestLowerConcat checks that lower wrapping concat compiles to
// LOWER(CONCAT(...)) with the separator bound.
func (s *ParserSuite) TestLowerConcat() {
	s.assertStatement(
		"lower(concat(first_name, ' ', last_name))",
		"LOWER(CONCAT(first_name, @p0, last_name))",
		map[string]any{"p0": " "},
	)
}

// TestConcatLower checks that concat of lower(...) arguments compiles to
// CONCAT(LOWER(...), @p0, LOWER(...)).
func (s *ParserSuite) TestConcatLower() {
	s.assertStatement(
		"concat(lower(first_name), ' ', lower(last_name))",
		"CONCAT(LOWER(first_name), @p0, LOWER(last_name))",
		map[string]any{"p0": " "},
	)
}

// =============================================================================
// Built-in Type Functions Tests
// =============================================================================

// TestTimestampFunction checks that timestamp('...') binds the string for
// PARSE_TIMESTAMP('%c',@p0) and rewrites the field to TIMESTAMP_ADD.
func (s *ParserSuite) TestTimestampFunction() {
	s.assertStatement(
		"create_time > timestamp('2021-01-01T00:00:00Z')",
		"TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p0)",
		map[string]any{"p0": "2021-01-01T00:00:00Z"},
	)
}

// TestTimestampFunctionLessThan checks that < works with timestamp('...') on
// the update_time Timestamp identifier, binding the RFC 3339 string.
func (s *ParserSuite) TestTimestampFunctionLessThan() {
	s.assertStatement(
		"update_time < timestamp('2025-12-31T23:59:59Z')",
		"TIMESTAMP_ADD(TIMESTAMP_SECONDS(update_time.seconds),INTERVAL CAST(FLOOR(IFNULL(update_time.nanos,0) / 1000) AS INT64) MICROSECOND) < PARSE_TIMESTAMP('%c',@p0)",
		map[string]any{"p0": "2025-12-31T23:59:59Z"},
	)
}

// TestDurationFunction checks that duration('1h') on a Duration identifier
// compares seconds plus nanos/1e9 against 3600 bound as float64.
func (s *ParserSuite) TestDurationFunction() {
	s.assertStatement(
		"expire_after > duration('1h')",
		"(expire_after.seconds + IFNULL(expire_after.nanos,0) / 1e9) > @p0",
		map[string]any{"p0": float64(3600)},
	)
}

// TestDurationFunctionMinutes checks that duration('30m') is converted to 1800
// seconds and bound as float64.
func (s *ParserSuite) TestDurationFunctionMinutes() {
	s.assertStatement(
		"timeout < duration('30m')",
		"(timeout.seconds + IFNULL(timeout.nanos,0) / 1e9) < @p0",
		map[string]any{"p0": float64(1800)},
	)
}

// TestDurationFunctionSeconds checks that duration('90s') is converted to 90
// seconds and bound as float64 under >=.
func (s *ParserSuite) TestDurationFunctionSeconds() {
	s.assertStatement(
		"expire_after >= duration('90s')",
		"(expire_after.seconds + IFNULL(expire_after.nanos,0) / 1e9) >= @p0",
		map[string]any{"p0": float64(90)},
	)
}

// TestDateFunction checks that date('...') on a Date identifier compiles to
// DATE(year, month, day) = DATE(@p0), binding the date string.
func (s *ParserSuite) TestDateFunction() {
	s.assertStatement(
		"effective_date == date('2021-01-01')",
		"DATE(effective_date.year, effective_date.month, effective_date.day) = DATE(@p0)",
		map[string]any{"p0": "2021-01-01"},
	)
}

// TestDateFunctionGreaterThan checks that > with date('...') on the birth_date
// Date identifier compiles to DATE(...) > DATE(@p0).
func (s *ParserSuite) TestDateFunctionGreaterThan() {
	s.assertStatement(
		"birth_date > date('1990-01-01')",
		"DATE(birth_date.year, birth_date.month, birth_date.day) > DATE(@p0)",
		map[string]any{"p0": "1990-01-01"},
	)
}

// =============================================================================
// IN Operator Tests
// =============================================================================

// TestInStringArray checks that in with a string list compiles to IN
// UNNEST(@p0) with a []string parameter.
func (s *ParserSuite) TestInStringArray() {
	s.assertStatement(
		"name in ['Alice', 'Bob', 'Charlie']",
		"name IN UNNEST(@p0)",
		map[string]any{"p0": []string{"Alice", "Bob", "Charlie"}},
	)
}

// TestInIntArray checks that in with an integer list compiles to IN UNNEST(@p0)
// with a []int64 parameter.
func (s *ParserSuite) TestInIntArray() {
	s.assertStatement(
		"age in [18, 21, 65]",
		"age IN UNNEST(@p0)",
		map[string]any{"p0": []int64{18, 21, 65}},
	)
}

// TestInSingleValue checks that a one-element list still compiles to IN
// UNNEST(@p0) with a one-element []string.
func (s *ParserSuite) TestInSingleValue() {
	// Using 'state' instead of 'status' since 'status' is registered as EnumString identifier
	s.assertStatement(
		"state in ['ACTIVE']",
		"state IN UNNEST(@p0)",
		map[string]any{"p0": []string{"ACTIVE"}},
	)
}

// TestInWithKeyword checks that the uppercase IN keyword is accepted and
// compiles to IN UNNEST(@p0).
func (s *ParserSuite) TestInWithKeyword() {
	s.assertStatement(
		"name IN ['Alice', 'Bob']",
		"name IN UNNEST(@p0)",
		map[string]any{"p0": []string{"Alice", "Bob"}},
	)
}

// =============================================================================
// Identifier Transformations Tests
// =============================================================================

// TestTimestampIdentifier checks that a field registered with Timestamp is
// rewritten to the TIMESTAMP_ADD seconds and nanos expression.
func (s *ParserSuite) TestTimestampIdentifier() {
	s.assertStatement(
		"create_time > timestamp('2021-01-01T00:00:00Z')",
		"TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p0)",
		map[string]any{"p0": "2021-01-01T00:00:00Z"},
	)
}

// TestDurationIdentifier checks that a Duration field is rewritten to seconds
// plus nanos/1e9 and that duration('2h30m') binds 9000.
func (s *ParserSuite) TestDurationIdentifier() {
	s.assertStatement(
		"expire_after > duration('2h30m')",
		"(expire_after.seconds + IFNULL(expire_after.nanos,0) / 1e9) > @p0",
		map[string]any{"p0": float64(9000)}, // 2.5 hours = 9000 seconds
	)
}

// TestDateIdentifier checks that a Date field is rewritten to DATE(year, month,
// day) and works with !=.
func (s *ParserSuite) TestDateIdentifier() {
	s.assertStatement(
		"effective_date != date('2020-12-31')",
		"DATE(effective_date.year, effective_date.month, effective_date.day) != DATE(@p0)",
		map[string]any{"p0": "2020-12-31"},
	)
}

// TestReservedIdentifier checks that a field registered with Reserved, such as
// select, is quoted with backticks.
func (s *ParserSuite) TestReservedIdentifier() {
	s.assertStatement(
		"select == 'value'",
		"`select` = @p0",
		map[string]any{"p0": "value"},
	)
}

// TestReservedIdentifierFrom checks that the Reserved identifier from is quoted
// as `from` in the SQL.
func (s *ParserSuite) TestReservedIdentifierFrom() {
	s.assertStatement(
		"from == 'source'",
		"`from` = @p0",
		map[string]any{"p0": "source"},
	)
}

// TestEnumStringIdentifier checks that an EnumString field is wrapped in
// CAST(status AS STRING) with the name bound.
func (s *ParserSuite) TestEnumStringIdentifier() {
	s.assertStatement(
		"status == 'ACTIVE'",
		"CAST(status AS STRING) = @p0",
		map[string]any{"p0": "ACTIVE"},
	)
}

// TestEnumIntegerIdentifier checks that an EnumInteger field is wrapped in
// CAST(priority AS INT64) with the number bound as int64.
func (s *ParserSuite) TestEnumIntegerIdentifier() {
	s.assertStatement(
		"priority == 1",
		"CAST(priority AS INT64) = @p0",
		map[string]any{"p0": int64(1)},
	)
}

// =============================================================================
// Nested Field Access (SelectExpr) Tests
// =============================================================================

// TestSelectExpr checks that a dotted field path such as Proto.field is emitted
// unchanged as the column.
func (s *ParserSuite) TestSelectExpr() {
	s.assertStatement(
		"Proto.field == 'value'",
		"Proto.field = @p0",
		map[string]any{"p0": "value"},
	)
}

// TestDeepSelectExpr checks that a three-level path such as user.address.city
// is emitted unchanged.
func (s *ParserSuite) TestDeepSelectExpr() {
	s.assertStatement(
		"user.address.city == 'NYC'",
		"user.address.city = @p0",
		map[string]any{"p0": "NYC"},
	)
}

// TestSelectExprWithComparison checks that a dotted field path works with > and
// an int64 parameter.
func (s *ParserSuite) TestSelectExprWithComparison() {
	s.assertStatement(
		"Proto.count > 10",
		"Proto.count > @p0",
		map[string]any{"p0": int64(10)},
	)
}

// TestSelectExprWithFunction checks that a dotted field path works as the first
// argument of like.
func (s *ParserSuite) TestSelectExprWithFunction() {
	s.assertStatement(
		"like(Proto.name, '%test%')",
		"Proto.name LIKE @p0",
		map[string]any{"p0": "%test%"},
	)
}

// TestSelectExprWithIn checks that a dotted field path works with in, compiling
// to Proto.state IN UNNEST(@p0).
func (s *ParserSuite) TestSelectExprWithIn() {
	s.assertStatement(
		"Proto.state in ['ACTIVE', 'PENDING']",
		"Proto.state IN UNNEST(@p0)",
		map[string]any{"p0": []string{"ACTIVE", "PENDING"}},
	)
}

// =============================================================================
// Error Cases Tests
// =============================================================================

// TestUnsupportedFunction checks that calling an unknown function returns a
// parse error.
func (s *ParserSuite) TestUnsupportedFunction() {
	s.assertError("unknownfunc(name)")
}

// TestInvalidFilterSyntax checks that a comparison missing its right operand
// returns a parse error.
func (s *ParserSuite) TestInvalidFilterSyntax() {
	s.assertError("name ==")
}

// TestInvalidFilterMissingOperand checks that a comparison missing its left
// operand returns a parse error.
func (s *ParserSuite) TestInvalidFilterMissingOperand() {
	s.assertError("== 'value'")
}

// TestInvalidFilterUnbalancedParens checks that an unclosed parenthesis returns
// a parse error.
func (s *ParserSuite) TestInvalidFilterUnbalancedParens() {
	s.assertError("(name == 'Alice'")
}

// =============================================================================
// Complex Integration Tests
// =============================================================================

// TestComplexFilterWithTimestamp checks that an equality ANDed with a Timestamp
// comparison binds the string as @p0 and the timestamp as @p1.
func (s *ParserSuite) TestComplexFilterWithTimestamp() {
	s.assertStatement(
		"name == 'Alice' AND create_time > timestamp('2021-01-01T00:00:00Z')",
		"(name = @p0 AND TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p1))",
		map[string]any{"p0": "Alice", "p1": "2021-01-01T00:00:00Z"},
	)
}

// TestNestedFunctions checks that like(lower(concat(...))) nests as
// LOWER(CONCAT(...)) LIKE @p0.
func (s *ParserSuite) TestNestedFunctions() {
	s.assertStatement(
		"like(lower(concat(first_name, last_name)), '%smith%')",
		"LOWER(CONCAT(first_name, last_name)) LIKE @p0",
		map[string]any{"p0": "%smith%"},
	)
}

// TestMixedOperators checks that a parenthesized OR ANDed with a comparison
// keeps the OR group nested inside the AND.
func (s *ParserSuite) TestMixedOperators() {
	s.assertStatement(
		"(name == 'Alice' OR name == 'Bob') AND age > 18",
		"((name = @p0 OR name = @p1) AND age > @p2)",
		map[string]any{"p0": "Alice", "p1": "Bob", "p2": int64(18)},
	)
}

// TestMultipleFunctions checks that prefix, suffix and a comparison joined by
// AND nest left to right with @p0 to @p2 in order.
func (s *ParserSuite) TestMultipleFunctions() {
	s.assertStatement(
		"prefix(name, 'Al') AND suffix(name, 'ce') AND age >= 18",
		"((STARTS_WITH(name, @p0) AND ENDS_WITH(name, @p1)) AND age >= @p2)",
		map[string]any{"p0": "Al", "p1": "ce", "p2": int64(18)},
	)
}

// TestMultipleIdentifierTypes checks that Timestamp and Date identifiers in one
// AND each get their own rewrite and parameter.
func (s *ParserSuite) TestMultipleIdentifierTypes() {
	s.assertStatement(
		"create_time > timestamp('2021-01-01T00:00:00Z') AND effective_date == date('2021-06-01')",
		"(TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p0) AND DATE(effective_date.year, effective_date.month, effective_date.day) = DATE(@p1))",
		map[string]any{"p0": "2021-01-01T00:00:00Z", "p1": "2021-06-01"},
	)
}

// TestRealWorldFilter checks a filter mixing EnumString, Timestamp, like over
// lower and EnumInteger, pinning the full SQL and @p0 to @p3.
func (s *ParserSuite) TestRealWorldFilter() {
	s.assertStatement(
		"status == 'ACTIVE' AND create_time > timestamp('2021-01-01T00:00:00Z') AND (like(lower(name), '%test%') OR priority == 1)",
		"((CAST(status AS STRING) = @p0 AND TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p1)) AND (LOWER(name) LIKE @p2 OR CAST(priority AS INT64) = @p3))",
		map[string]any{"p0": "ACTIVE", "p1": "2021-01-01T00:00:00Z", "p2": "%test%", "p3": int64(1)},
	)
}

// TestFilterWithCoalesceAndComparison checks that a coalesce call can be the
// left side of an equality.
func (s *ParserSuite) TestFilterWithCoalesceAndComparison() {
	s.assertStatement(
		"coalesce(nickname, name) == 'Alice'",
		"COALESCE(nickname, name) = @p0",
		map[string]any{"p0": "Alice"},
	)
}

// TestFilterWithGreatestComparison checks that a greatest call can be the left
// side of a > comparison.
func (s *ParserSuite) TestFilterWithGreatestComparison() {
	s.assertStatement(
		"greatest(score1, score2) > 90",
		"GREATEST(score1, score2) > @p0",
		map[string]any{"p0": int64(90)},
	)
}

// TestFilterWithLeastComparison checks that a least call can be the left side
// of a < comparison.
func (s *ParserSuite) TestFilterWithLeastComparison() {
	s.assertStatement(
		"least(price, max_price) < 100",
		"LEAST(price, max_price) < @p0",
		map[string]any{"p0": int64(100)},
	)
}

// TestFilterWithIfnullComparison checks that the ifnull default and the
// compared value are bound as separate parameters @p0 and @p1.
func (s *ParserSuite) TestFilterWithIfnullComparison() {
	s.assertStatement(
		"ifnull(discount, 0) > 10",
		"IFNULL(discount, @p0) > @p1",
		map[string]any{"p0": int64(0), "p1": int64(10)},
	)
}

// TestComplexNestedConditions checks that two parenthesized AND groups joined
// by || keep both groups nested inside the OR.
func (s *ParserSuite) TestComplexNestedConditions() {
	s.assertStatement(
		"(a == 1 && b == 2) || (c == 3 && d == 4)",
		"((a = @p0 AND b = @p1) OR (c = @p2 AND d = @p3))",
		map[string]any{"p0": int64(1), "p1": int64(2), "p2": int64(3), "p3": int64(4)},
	)
}

// TestFilterWithInAndComparison checks that an in list ANDed with a comparison
// binds the []string as @p0 and the integer as @p1.
func (s *ParserSuite) TestFilterWithInAndComparison() {
	s.assertStatement(
		"name in ['Alice', 'Bob'] AND age > 18",
		"(name IN UNNEST(@p0) AND age > @p1)",
		map[string]any{"p0": []string{"Alice", "Bob"}, "p1": int64(18)},
	)
}

// TestFilterWithSelectExprAndFunctions checks that dotted field paths work
// inside lower and like and in a comparison within one AND.
func (s *ParserSuite) TestFilterWithSelectExprAndFunctions() {
	s.assertStatement(
		"like(lower(Proto.name), '%test%') AND Proto.count > 0",
		"(LOWER(Proto.name) LIKE @p0 AND Proto.count > @p1)",
		map[string]any{"p0": "%test%", "p1": int64(0)},
	)
}

// =============================================================================
// NULL Handling Tests
// =============================================================================

// assertStatementWith is assertStatement against a parser built from
// identifiers, for cases that must not alter the shared suite parser.
func (s *ParserSuite) assertStatementWith(identifiers []Identifier, filter, expectedSQL string, expectedParams map[string]any) {
	p, err := NewParser(identifiers...)
	s.Require().NoError(err)
	stmt, err := p.Parse(filter)
	s.Require().NoError(err, filter)
	s.Equal(expectedSQL, stmt.SQL, filter)
	s.Equal(expectedParams, stmt.Params, filter)
}

// TestNullEquality checks that == null compiles to IS NULL with no parameter.
func (s *ParserSuite) TestNullEquality() {
	s.assertStatement("name == null", "name IS NULL", map[string]any{})
}

// TestNullInequality checks that != null compiles to IS NOT NULL with no
// parameter.
func (s *ParserSuite) TestNullInequality() {
	s.assertStatement("name != null", "name IS NOT NULL", map[string]any{})
}

// TestNullKeyword checks that NULL is case-insensitive and that the single =
// operator also yields IS NULL.
func (s *ParserSuite) TestNullKeyword() {
	s.assertStatement("name == NULL", "name IS NULL", map[string]any{})
	s.assertStatement("name = NULL", "name IS NULL", map[string]any{})
	s.assertStatement("name = null", "name IS NULL", map[string]any{})
}

// TestNullOnLeft checks that null on the left side is swapped so the field
// comes first in IS NULL and IS NOT NULL.
func (s *ParserSuite) TestNullOnLeft() {
	s.assertStatement("null == name", "name IS NULL", map[string]any{})
	s.assertStatement("NULL != name", "name IS NOT NULL", map[string]any{})
}

// TestNullBothSides checks that comparing null with null compiles to NULL IS
// NULL and NULL IS NOT NULL.
func (s *ParserSuite) TestNullBothSides() {
	s.assertStatement("null == null", "NULL IS NULL", map[string]any{})
	s.assertStatement("null != null", "NULL IS NOT NULL", map[string]any{})
}

// A constant compared with null is still bound, never spliced into SQL.
func (s *ParserSuite) TestNullAgainstConstantStaysBound() {
	s.assertStatement("null == 'x OR TRUE'", "@p0 IS NULL", map[string]any{"p0": "x OR TRUE"})
	s.assertStatement("1 != null", "@p0 IS NOT NULL", map[string]any{"p0": int64(1)})
}

// TestNullOnTimestampIdentifier checks that = NULL on a Timestamp identifier
// applies IS NULL to the rewritten TIMESTAMP_ADD expression.
func (s *ParserSuite) TestNullOnTimestampIdentifier() {
	s.assertStatementWith(
		[]Identifier{Timestamp("Proto.delete_time")},
		"Proto.delete_time = NULL",
		"TIMESTAMP_ADD(TIMESTAMP_SECONDS(Proto.delete_time.seconds),INTERVAL CAST(FLOOR(IFNULL(Proto.delete_time.nanos,0) / 1000) AS INT64) MICROSECOND) IS NULL",
		map[string]any{},
	)
}

// TestNullCombinedWithEnum checks that an EnumString equality ANDed with a
// Timestamp = NULL check binds only the enum value.
func (s *ParserSuite) TestNullCombinedWithEnum() {
	s.assertStatementWith(
		[]Identifier{Timestamp("Proto.delete_time"), EnumString("Proto.state", "test.State")},
		"Proto.state = 'ACTIVE' AND Proto.delete_time = NULL",
		"(CAST(Proto.state AS STRING) = @p0 AND TIMESTAMP_ADD(TIMESTAMP_SECONDS(Proto.delete_time.seconds),INTERVAL CAST(FLOOR(IFNULL(Proto.delete_time.nanos,0) / 1000) AS INT64) MICROSECOND) IS NULL)",
		map[string]any{"p0": "ACTIVE"},
	)
}

// TestNullInOrderingComparisonIsInvalid checks that null with >, >=, < or <= on
// either side returns ErrInvalidFilter.
func (s *ParserSuite) TestNullInOrderingComparisonIsInvalid() {
	for _, f := range []string{"name > null", "name >= null", "name < NULL", "name <= null", "null < name"} {
		_, err := s.parser.Parse(f)
		var invalid ErrInvalidFilter
		s.Require().ErrorAs(err, &invalid, f)
	}
}

// TestNullStringLiteralStillBinds checks that the quoted string 'NULL' is bound
// as a parameter, not treated as SQL NULL.
func (s *ParserSuite) TestNullStringLiteralStillBinds() {
	s.assertStatement("name = 'NULL'", "name = @p0", map[string]any{"p0": "NULL"})
}

// TestNullInsideIfnullUnchanged checks that null handling leaves IFNULL alone:
// its default and the compared value stay bound.
func (s *ParserSuite) TestNullInsideIfnullUnchanged() {
	s.assertStatement("IFNULL(nickname, 'N/A') = 'x'", "IFNULL(nickname, @p0) = @p1", map[string]any{"p0": "N/A", "p1": "x"})
}

// =============================================================================
// Case Insensitivity Tests (Function Names)
// =============================================================================

// TestUppercaseLike checks that the function name LIKE is accepted in uppercase
// and compiles like like.
func (s *ParserSuite) TestUppercaseLike() {
	s.assertStatement(
		"LIKE(name, '%test%')",
		"name LIKE @p0",
		map[string]any{"p0": "%test%"},
	)
}

// TestUppercaseLower checks that the function name LOWER is accepted in
// uppercase and compiles like lower.
func (s *ParserSuite) TestUppercaseLower() {
	s.assertStatement(
		"LOWER(name) == 'test'",
		"LOWER(name) = @p0",
		map[string]any{"p0": "test"},
	)
}

// TestUppercaseUpper checks that the function name UPPER is accepted in
// uppercase and compiles like upper.
func (s *ParserSuite) TestUppercaseUpper() {
	s.assertStatement(
		"UPPER(name) == 'TEST'",
		"UPPER(name) = @p0",
		map[string]any{"p0": "TEST"},
	)
}

// TestUppercasePrefix checks that PREFIX is accepted in uppercase and compiles
// to STARTS_WITH.
func (s *ParserSuite) TestUppercasePrefix() {
	s.assertStatement(
		"PREFIX(name, 'Al')",
		"STARTS_WITH(name, @p0)",
		map[string]any{"p0": "Al"},
	)
}

// TestUppercaseSuffix checks that SUFFIX is accepted in uppercase and compiles
// to ENDS_WITH.
func (s *ParserSuite) TestUppercaseSuffix() {
	s.assertStatement(
		"SUFFIX(name, 'ce')",
		"ENDS_WITH(name, @p0)",
		map[string]any{"p0": "ce"},
	)
}

// TestUppercaseConcat checks that CONCAT is accepted in uppercase and still
// binds the string separator.
func (s *ParserSuite) TestUppercaseConcat() {
	s.assertStatement(
		"CONCAT(first, ' ', last)",
		"CONCAT(first, @p0, last)",
		map[string]any{"p0": " "},
	)
}

// TestUppercaseGreatest checks that GREATEST is accepted in uppercase and
// compiles to GREATEST(a, b).
func (s *ParserSuite) TestUppercaseGreatest() {
	s.assertStatement(
		"GREATEST(a, b)",
		"GREATEST(a, b)",
		map[string]any{},
	)
}

// TestUppercaseLeast checks that LEAST is accepted in uppercase and compiles to
// LEAST(a, b).
func (s *ParserSuite) TestUppercaseLeast() {
	s.assertStatement(
		"LEAST(a, b)",
		"LEAST(a, b)",
		map[string]any{},
	)
}

// TestUppercaseCoalesce checks that COALESCE is accepted in uppercase and
// compiles to COALESCE(a, b).
func (s *ParserSuite) TestUppercaseCoalesce() {
	s.assertStatement(
		"COALESCE(a, b)",
		"COALESCE(a, b)",
		map[string]any{},
	)
}

// TestUppercaseIfnull checks that IFNULL is accepted in uppercase and compiles
// to IFNULL(a, b).
func (s *ParserSuite) TestUppercaseIfnull() {
	s.assertStatement(
		"IFNULL(a, b)",
		"IFNULL(a, b)",
		map[string]any{},
	)
}
