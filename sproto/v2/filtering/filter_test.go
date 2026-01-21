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

func (s *ParserSuite) TestEquality() {
	s.assertStatement(
		"name == 'Alice'",
		"name = @p0",
		map[string]any{"p0": "Alice"},
	)
}

func (s *ParserSuite) TestEqualityWithInt() {
	s.assertStatement(
		"age == 25",
		"age = @p0",
		map[string]any{"p0": int64(25)},
	)
}

func (s *ParserSuite) TestEqualityWithBool() {
	s.assertStatement(
		"active == true",
		"active = @p0",
		map[string]any{"p0": true},
	)
}

func (s *ParserSuite) TestInequality() {
	s.assertStatement(
		"name != 'Alice'",
		"name != @p0",
		map[string]any{"p0": "Alice"},
	)
}

func (s *ParserSuite) TestGreaterThan() {
	s.assertStatement(
		"age > 18",
		"age > @p0",
		map[string]any{"p0": int64(18)},
	)
}

func (s *ParserSuite) TestGreaterThanOrEqual() {
	s.assertStatement(
		"age >= 18",
		"age >= @p0",
		map[string]any{"p0": int64(18)},
	)
}

func (s *ParserSuite) TestLessThan() {
	s.assertStatement(
		"age < 65",
		"age < @p0",
		map[string]any{"p0": int64(65)},
	)
}

func (s *ParserSuite) TestLessThanOrEqual() {
	s.assertStatement(
		"age <= 65",
		"age <= @p0",
		map[string]any{"p0": int64(65)},
	)
}

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

func (s *ParserSuite) TestLogicalAnd() {
	s.assertStatement(
		"a == 1 && b == 2",
		"(a = @p0 AND b = @p1)",
		map[string]any{"p0": int64(1), "p1": int64(2)},
	)
}

func (s *ParserSuite) TestLogicalAndKeyword() {
	s.assertStatement(
		"a == 1 AND b == 2",
		"(a = @p0 AND b = @p1)",
		map[string]any{"p0": int64(1), "p1": int64(2)},
	)
}

func (s *ParserSuite) TestLogicalOr() {
	s.assertStatement(
		"a == 1 || b == 2",
		"(a = @p0 OR b = @p1)",
		map[string]any{"p0": int64(1), "p1": int64(2)},
	)
}

func (s *ParserSuite) TestLogicalOrKeyword() {
	s.assertStatement(
		"a == 1 OR b == 2",
		"(a = @p0 OR b = @p1)",
		map[string]any{"p0": int64(1), "p1": int64(2)},
	)
}

func (s *ParserSuite) TestNestedLogical() {
	s.assertStatement(
		"(a == 1 && b == 2) || c == 3",
		"((a = @p0 AND b = @p1) OR c = @p2)",
		map[string]any{"p0": int64(1), "p1": int64(2), "p2": int64(3)},
	)
}

func (s *ParserSuite) TestComplexLogical() {
	s.assertStatement(
		"a == 1 && (b == 2 || c == 3)",
		"(a = @p0 AND (b = @p1 OR c = @p2))",
		map[string]any{"p0": int64(1), "p1": int64(2), "p2": int64(3)},
	)
}

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

func (s *ParserSuite) TestLike() {
	s.assertStatement(
		"like(name, '%Alice%')",
		"name LIKE @p0",
		map[string]any{"p0": "%Alice%"},
	)
}

func (s *ParserSuite) TestLikeStartsWith() {
	s.assertStatement(
		"like(name, 'Alice%')",
		"name LIKE @p0",
		map[string]any{"p0": "Alice%"},
	)
}

func (s *ParserSuite) TestLikeEndsWith() {
	s.assertStatement(
		"like(name, '%Alice')",
		"name LIKE @p0",
		map[string]any{"p0": "%Alice"},
	)
}

func (s *ParserSuite) TestLower() {
	s.assertStatement(
		"lower(name) == 'alice'",
		"LOWER(name) = @p0",
		map[string]any{"p0": "alice"},
	)
}

func (s *ParserSuite) TestUpper() {
	s.assertStatement(
		"upper(name) == 'ALICE'",
		"UPPER(name) = @p0",
		map[string]any{"p0": "ALICE"},
	)
}

func (s *ParserSuite) TestPrefix() {
	s.assertStatement(
		"prefix(name, 'Al')",
		"STARTS_WITH(name, @p0)",
		map[string]any{"p0": "Al"},
	)
}

func (s *ParserSuite) TestSuffix() {
	s.assertStatement(
		"suffix(name, 'ce')",
		"ENDS_WITH(name, @p0)",
		map[string]any{"p0": "ce"},
	)
}

func (s *ParserSuite) TestLikeLower() {
	s.assertStatement(
		"like(lower(name), '%alice%')",
		"LOWER(name) LIKE @p0",
		map[string]any{"p0": "%alice%"},
	)
}

func (s *ParserSuite) TestLikeUpper() {
	s.assertStatement(
		"like(upper(name), '%ALICE%')",
		"UPPER(name) LIKE @p0",
		map[string]any{"p0": "%ALICE%"},
	)
}

func (s *ParserSuite) TestPrefixWithLower() {
	s.assertStatement(
		"prefix(lower(name), 'al')",
		"STARTS_WITH(LOWER(name), @p0)",
		map[string]any{"p0": "al"},
	)
}

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

func (s *ParserSuite) TestConcat() {
	s.assertStatement(
		"concat(first_name, ' ', last_name)",
		"CONCAT(first_name, @p0, last_name)",
		map[string]any{"p0": " "},
	)
}

func (s *ParserSuite) TestConcatTwoFields() {
	s.assertStatement(
		"concat(first_name, last_name)",
		"CONCAT(first_name, last_name)",
		map[string]any{},
	)
}

func (s *ParserSuite) TestConcatWithConstant() {
	s.assertStatement(
		"concat('Hello ', name)",
		"CONCAT(@p0, name)",
		map[string]any{"p0": "Hello "},
	)
}

func (s *ParserSuite) TestGreatest() {
	s.assertStatement(
		"greatest(a, b, 10)",
		"GREATEST(a, b, @p0)",
		map[string]any{"p0": int64(10)},
	)
}

func (s *ParserSuite) TestGreatestTwoFields() {
	s.assertStatement(
		"greatest(price, min_price)",
		"GREATEST(price, min_price)",
		map[string]any{},
	)
}

func (s *ParserSuite) TestLeast() {
	s.assertStatement(
		"least(a, b, 10)",
		"LEAST(a, b, @p0)",
		map[string]any{"p0": int64(10)},
	)
}

func (s *ParserSuite) TestLeastTwoFields() {
	s.assertStatement(
		"least(quantity, max_quantity)",
		"LEAST(quantity, max_quantity)",
		map[string]any{},
	)
}

func (s *ParserSuite) TestCoalesce() {
	s.assertStatement(
		"coalesce(nickname, name, 'Unknown')",
		"COALESCE(nickname, name, @p0)",
		map[string]any{"p0": "Unknown"},
	)
}

func (s *ParserSuite) TestCoalesceTwoFields() {
	s.assertStatement(
		"coalesce(nickname, name)",
		"COALESCE(nickname, name)",
		map[string]any{},
	)
}

func (s *ParserSuite) TestIfnull() {
	s.assertStatement(
		"ifnull(nickname, 'N/A')",
		"IFNULL(nickname, @p0)",
		map[string]any{"p0": "N/A"},
	)
}

func (s *ParserSuite) TestIfnullWithInt() {
	s.assertStatement(
		"ifnull(count, 0)",
		"IFNULL(count, @p0)",
		map[string]any{"p0": int64(0)},
	)
}

func (s *ParserSuite) TestIfnullTwoFields() {
	s.assertStatement(
		"ifnull(primary_email, secondary_email)",
		"IFNULL(primary_email, secondary_email)",
		map[string]any{},
	)
}

func (s *ParserSuite) TestLowerConcat() {
	s.assertStatement(
		"lower(concat(first_name, ' ', last_name))",
		"LOWER(CONCAT(first_name, @p0, last_name))",
		map[string]any{"p0": " "},
	)
}

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

func (s *ParserSuite) TestTimestampFunction() {
	s.assertStatement(
		"create_time > timestamp('2021-01-01T00:00:00Z')",
		"TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p0)",
		map[string]any{"p0": "2021-01-01T00:00:00Z"},
	)
}

func (s *ParserSuite) TestTimestampFunctionLessThan() {
	s.assertStatement(
		"update_time < timestamp('2025-12-31T23:59:59Z')",
		"TIMESTAMP_ADD(TIMESTAMP_SECONDS(update_time.seconds),INTERVAL CAST(FLOOR(IFNULL(update_time.nanos,0) / 1000) AS INT64) MICROSECOND) < PARSE_TIMESTAMP('%c',@p0)",
		map[string]any{"p0": "2025-12-31T23:59:59Z"},
	)
}

func (s *ParserSuite) TestDurationFunction() {
	s.assertStatement(
		"expire_after > duration('1h')",
		"(expire_after.seconds + IFNULL(expire_after.nanos,0) / 1e9) > @p0",
		map[string]any{"p0": float64(3600)},
	)
}

func (s *ParserSuite) TestDurationFunctionMinutes() {
	s.assertStatement(
		"timeout < duration('30m')",
		"(timeout.seconds + IFNULL(timeout.nanos,0) / 1e9) < @p0",
		map[string]any{"p0": float64(1800)},
	)
}

func (s *ParserSuite) TestDurationFunctionSeconds() {
	s.assertStatement(
		"expire_after >= duration('90s')",
		"(expire_after.seconds + IFNULL(expire_after.nanos,0) / 1e9) >= @p0",
		map[string]any{"p0": float64(90)},
	)
}

func (s *ParserSuite) TestDateFunction() {
	s.assertStatement(
		"effective_date == date('2021-01-01')",
		"DATE(effective_date.year, effective_date.month, effective_date.day) = DATE(@p0)",
		map[string]any{"p0": "2021-01-01"},
	)
}

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

func (s *ParserSuite) TestInStringArray() {
	s.assertStatement(
		"name in ['Alice', 'Bob', 'Charlie']",
		"name IN UNNEST(@p0)",
		map[string]any{"p0": []string{"Alice", "Bob", "Charlie"}},
	)
}

func (s *ParserSuite) TestInIntArray() {
	s.assertStatement(
		"age in [18, 21, 65]",
		"age IN UNNEST(@p0)",
		map[string]any{"p0": []int64{18, 21, 65}},
	)
}

func (s *ParserSuite) TestInSingleValue() {
	// Using 'state' instead of 'status' since 'status' is registered as EnumString identifier
	s.assertStatement(
		"state in ['ACTIVE']",
		"state IN UNNEST(@p0)",
		map[string]any{"p0": []string{"ACTIVE"}},
	)
}

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

func (s *ParserSuite) TestTimestampIdentifier() {
	s.assertStatement(
		"create_time > timestamp('2021-01-01T00:00:00Z')",
		"TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p0)",
		map[string]any{"p0": "2021-01-01T00:00:00Z"},
	)
}

func (s *ParserSuite) TestDurationIdentifier() {
	s.assertStatement(
		"expire_after > duration('2h30m')",
		"(expire_after.seconds + IFNULL(expire_after.nanos,0) / 1e9) > @p0",
		map[string]any{"p0": float64(9000)}, // 2.5 hours = 9000 seconds
	)
}

func (s *ParserSuite) TestDateIdentifier() {
	s.assertStatement(
		"effective_date != date('2020-12-31')",
		"DATE(effective_date.year, effective_date.month, effective_date.day) != DATE(@p0)",
		map[string]any{"p0": "2020-12-31"},
	)
}

func (s *ParserSuite) TestReservedIdentifier() {
	s.assertStatement(
		"select == 'value'",
		"`select` = @p0",
		map[string]any{"p0": "value"},
	)
}

func (s *ParserSuite) TestReservedIdentifierFrom() {
	s.assertStatement(
		"from == 'source'",
		"`from` = @p0",
		map[string]any{"p0": "source"},
	)
}

func (s *ParserSuite) TestEnumStringIdentifier() {
	s.assertStatement(
		"status == 'ACTIVE'",
		"CAST(status AS STRING) = @p0",
		map[string]any{"p0": "ACTIVE"},
	)
}

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

func (s *ParserSuite) TestSelectExpr() {
	s.assertStatement(
		"Proto.field == 'value'",
		"Proto.field = @p0",
		map[string]any{"p0": "value"},
	)
}

func (s *ParserSuite) TestDeepSelectExpr() {
	s.assertStatement(
		"user.address.city == 'NYC'",
		"user.address.city = @p0",
		map[string]any{"p0": "NYC"},
	)
}

func (s *ParserSuite) TestSelectExprWithComparison() {
	s.assertStatement(
		"Proto.count > 10",
		"Proto.count > @p0",
		map[string]any{"p0": int64(10)},
	)
}

func (s *ParserSuite) TestSelectExprWithFunction() {
	s.assertStatement(
		"like(Proto.name, '%test%')",
		"Proto.name LIKE @p0",
		map[string]any{"p0": "%test%"},
	)
}

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

func (s *ParserSuite) TestUnsupportedFunction() {
	s.assertError("unknownfunc(name)")
}

func (s *ParserSuite) TestInvalidFilterSyntax() {
	s.assertError("name ==")
}

func (s *ParserSuite) TestInvalidFilterMissingOperand() {
	s.assertError("== 'value'")
}

func (s *ParserSuite) TestInvalidFilterUnbalancedParens() {
	s.assertError("(name == 'Alice'")
}

// =============================================================================
// Complex Integration Tests
// =============================================================================

func (s *ParserSuite) TestComplexFilterWithTimestamp() {
	s.assertStatement(
		"name == 'Alice' AND create_time > timestamp('2021-01-01T00:00:00Z')",
		"(name = @p0 AND TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p1))",
		map[string]any{"p0": "Alice", "p1": "2021-01-01T00:00:00Z"},
	)
}

func (s *ParserSuite) TestNestedFunctions() {
	s.assertStatement(
		"like(lower(concat(first_name, last_name)), '%smith%')",
		"LOWER(CONCAT(first_name, last_name)) LIKE @p0",
		map[string]any{"p0": "%smith%"},
	)
}

func (s *ParserSuite) TestMixedOperators() {
	s.assertStatement(
		"(name == 'Alice' OR name == 'Bob') AND age > 18",
		"((name = @p0 OR name = @p1) AND age > @p2)",
		map[string]any{"p0": "Alice", "p1": "Bob", "p2": int64(18)},
	)
}

func (s *ParserSuite) TestMultipleFunctions() {
	s.assertStatement(
		"prefix(name, 'Al') AND suffix(name, 'ce') AND age >= 18",
		"((STARTS_WITH(name, @p0) AND ENDS_WITH(name, @p1)) AND age >= @p2)",
		map[string]any{"p0": "Al", "p1": "ce", "p2": int64(18)},
	)
}

func (s *ParserSuite) TestMultipleIdentifierTypes() {
	s.assertStatement(
		"create_time > timestamp('2021-01-01T00:00:00Z') AND effective_date == date('2021-06-01')",
		"(TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p0) AND DATE(effective_date.year, effective_date.month, effective_date.day) = DATE(@p1))",
		map[string]any{"p0": "2021-01-01T00:00:00Z", "p1": "2021-06-01"},
	)
}

func (s *ParserSuite) TestRealWorldFilter() {
	s.assertStatement(
		"status == 'ACTIVE' AND create_time > timestamp('2021-01-01T00:00:00Z') AND (like(lower(name), '%test%') OR priority == 1)",
		"((CAST(status AS STRING) = @p0 AND TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p1)) AND (LOWER(name) LIKE @p2 OR CAST(priority AS INT64) = @p3))",
		map[string]any{"p0": "ACTIVE", "p1": "2021-01-01T00:00:00Z", "p2": "%test%", "p3": int64(1)},
	)
}

func (s *ParserSuite) TestFilterWithCoalesceAndComparison() {
	s.assertStatement(
		"coalesce(nickname, name) == 'Alice'",
		"COALESCE(nickname, name) = @p0",
		map[string]any{"p0": "Alice"},
	)
}

func (s *ParserSuite) TestFilterWithGreatestComparison() {
	s.assertStatement(
		"greatest(score1, score2) > 90",
		"GREATEST(score1, score2) > @p0",
		map[string]any{"p0": int64(90)},
	)
}

func (s *ParserSuite) TestFilterWithLeastComparison() {
	s.assertStatement(
		"least(price, max_price) < 100",
		"LEAST(price, max_price) < @p0",
		map[string]any{"p0": int64(100)},
	)
}

func (s *ParserSuite) TestFilterWithIfnullComparison() {
	s.assertStatement(
		"ifnull(discount, 0) > 10",
		"IFNULL(discount, @p0) > @p1",
		map[string]any{"p0": int64(0), "p1": int64(10)},
	)
}

func (s *ParserSuite) TestComplexNestedConditions() {
	s.assertStatement(
		"(a == 1 && b == 2) || (c == 3 && d == 4)",
		"((a = @p0 AND b = @p1) OR (c = @p2 AND d = @p3))",
		map[string]any{"p0": int64(1), "p1": int64(2), "p2": int64(3), "p3": int64(4)},
	)
}

func (s *ParserSuite) TestFilterWithInAndComparison() {
	s.assertStatement(
		"name in ['Alice', 'Bob'] AND age > 18",
		"(name IN UNNEST(@p0) AND age > @p1)",
		map[string]any{"p0": []string{"Alice", "Bob"}, "p1": int64(18)},
	)
}

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

func (s *ParserSuite) TestNullEquality() {
	s.assertStatement(
		"name == null",
		"name = @p0",
		map[string]any{"p0": "NULL"},
	)
}

func (s *ParserSuite) TestNullInequality() {
	s.assertStatement(
		"name != null",
		"name != @p0",
		map[string]any{"p0": "NULL"},
	)
}

func (s *ParserSuite) TestNullKeyword() {
	s.assertStatement(
		"name == NULL",
		"name = @p0",
		map[string]any{"p0": "NULL"},
	)
}

// =============================================================================
// Case Insensitivity Tests (Function Names)
// =============================================================================

func (s *ParserSuite) TestUppercaseLike() {
	s.assertStatement(
		"LIKE(name, '%test%')",
		"name LIKE @p0",
		map[string]any{"p0": "%test%"},
	)
}

func (s *ParserSuite) TestUppercaseLower() {
	s.assertStatement(
		"LOWER(name) == 'test'",
		"LOWER(name) = @p0",
		map[string]any{"p0": "test"},
	)
}

func (s *ParserSuite) TestUppercaseUpper() {
	s.assertStatement(
		"UPPER(name) == 'TEST'",
		"UPPER(name) = @p0",
		map[string]any{"p0": "TEST"},
	)
}

func (s *ParserSuite) TestUppercasePrefix() {
	s.assertStatement(
		"PREFIX(name, 'Al')",
		"STARTS_WITH(name, @p0)",
		map[string]any{"p0": "Al"},
	)
}

func (s *ParserSuite) TestUppercaseSuffix() {
	s.assertStatement(
		"SUFFIX(name, 'ce')",
		"ENDS_WITH(name, @p0)",
		map[string]any{"p0": "ce"},
	)
}

func (s *ParserSuite) TestUppercaseConcat() {
	s.assertStatement(
		"CONCAT(first, ' ', last)",
		"CONCAT(first, @p0, last)",
		map[string]any{"p0": " "},
	)
}

func (s *ParserSuite) TestUppercaseGreatest() {
	s.assertStatement(
		"GREATEST(a, b)",
		"GREATEST(a, b)",
		map[string]any{},
	)
}

func (s *ParserSuite) TestUppercaseLeast() {
	s.assertStatement(
		"LEAST(a, b)",
		"LEAST(a, b)",
		map[string]any{},
	)
}

func (s *ParserSuite) TestUppercaseCoalesce() {
	s.assertStatement(
		"COALESCE(a, b)",
		"COALESCE(a, b)",
		map[string]any{},
	)
}

func (s *ParserSuite) TestUppercaseIfnull() {
	s.assertStatement(
		"IFNULL(a, b)",
		"IFNULL(a, b)",
		map[string]any{},
	)
}
