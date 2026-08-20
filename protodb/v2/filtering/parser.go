package filtering

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// parseState carries the per-call state threaded through a single parseExpr
// walk: the accumulated Spanner query parameters (@p0, @p1, ...) and the
// caller-supplied named parameters available to param('name') lookups.
//
// A parseState is created fresh for each Parse call and passed by pointer
// through the recursive walk. It must never be stored on the Parser struct
// itself: Parser instances are shared across goroutines/requests, and
// per-call data (especially caller-supplied params) must not leak between
// concurrent Parse calls.
type parseState struct {
	params       map[string]any // accumulated Spanner query parameters, keyed p0, p1, ...
	callerParams map[string]any // caller-supplied named parameters for param('name'), nil if none were provided
}

// parseExpr recursively converts a CEL expression AST into SQL string and parameters.
//
// Parameters:
//   - expression: The CEL expression AST node to convert
//   - state: Per-call parse state (accumulated params + caller-supplied params).
//     A fresh state is created if nil; its params map is created if nil.
//
// Returns:
//   - sql: The generated SQL fragment (typically a string)
//   - params: Updated map of parameter names to values (e.g., {"p0": "Alice", "p1": 18})
//   - isFunction: True if this expression is a built-in function (timestamp, duration, date,
//     param) that should be embedded directly in SQL rather than parameterized
//   - error: Any parsing error encountered
//
// The function handles the following expression types:
//   - CallExpr: Function calls and operators (==, !=, >, AND, OR, like, etc.)
//   - IdentExpr: Identifiers/column names
//   - ConstExpr: Constant values (strings, numbers, booleans, null)
//   - SelectExpr: Field selection (e.g., Proto.field)
//   - ListExpr: List literals for IN operator
//   - StructExpr: Struct/map literals
func (f *Parser) parseExpr(expression *expr.Expr, state *parseState) (any, map[string]any, bool, error) {
	if state == nil {
		state = &parseState{}
	}
	if state.params == nil {
		state.params = make(map[string]any)
	}
	params := state.params

	switch expression.GetExprKind().(type) {
	case *expr.Expr_CallExpr:
		call := expression.GetCallExpr()

		switch call.Function {
		case "_&&_":
			leftSQL, leftParams, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, rightParams, _, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}
			for k, v := range leftParams {
				params[k] = v
			}
			for k, v := range rightParams {
				params[k] = v
			}
			return fmt.Sprintf("(%s AND %s)", leftSQL, rightSQL), params, false, nil
		case "_||_":
			leftSQL, leftParams, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, rightParams, _, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}
			for k, v := range leftParams {
				params[k] = v
			}
			for k, v := range rightParams {
				params[k] = v
			}
			return fmt.Sprintf("(%s OR %s)", leftSQL, rightSQL), params, false, nil
		case "_>_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL.(string))

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s > %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			return fmt.Sprintf("%s > @%s", leftSQL, paramName), params, false, nil
		case "_>=_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL.(string))

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s >= %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			return fmt.Sprintf("%s >= @%s", leftSQL, paramName), params, false, nil

		case "_<_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL.(string))

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s < %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			return fmt.Sprintf("%s < @%s", leftSQL, paramName), params, false, nil
		case "_<=_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL.(string))

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s <= %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			return fmt.Sprintf("%s <= @%s", leftSQL, paramName), params, false, nil
		case "_==_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL.(string))

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s = %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			return fmt.Sprintf("%s = @%s", leftSQL, paramName), params, false, nil
		case "_!=_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL.(string))

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s != %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			// TODO: Handle comparison of different types e.g NULL, FALSE
			return fmt.Sprintf("%s != @%s", leftSQL, paramName), params, false, nil
		case "timestamp", "TIMESTAMP":
			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = call.Args[0].GetConstExpr().GetStringValue()

			return fmt.Sprintf("PARSE_TIMESTAMP('%%c',@%s)", paramName), params, true, nil
		case "duration", "DURATION":
			durationStr := call.Args[0].GetConstExpr().GetStringValue()
			duration, err := time.ParseDuration(durationStr)
			if err != nil {
				return "", nil, false, err
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = duration.Seconds()

			return fmt.Sprintf("@%s", paramName), params, true, nil
		case "date", "DATE":
			dateStr := call.Args[0].GetConstExpr().GetStringValue()

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = dateStr

			return fmt.Sprintf("DATE(@%s)", paramName), params, true, nil
		case "param":
			if len(call.Args) != 1 {
				return "", nil, false, fmt.Errorf("param() requires exactly one string literal argument")
			}

			constExpr := call.Args[0].GetConstExpr()
			if constExpr == nil {
				return "", nil, false, fmt.Errorf("param() requires exactly one string literal argument")
			}

			strVal, ok := constExpr.ConstantKind.(*expr.Constant_StringValue)
			if !ok {
				return "", nil, false, fmt.Errorf("param() requires exactly one string literal argument")
			}
			name := strVal.StringValue

			if state.callerParams == nil {
				return "", nil, false, fmt.Errorf("param(%q) referenced but no params were provided to Parse", name)
			}

			value, ok := state.callerParams[name]
			if !ok {
				return "", nil, false, fmt.Errorf("param(%q) not found in provided params", name)
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = value

			// Bound caller params are already resolved values (not raw SQL/CEL
			// literals), so mark this as a "function" result: comparison
			// operators embed the @pN placeholder directly rather than
			// wrapping it in a second parameter.
			return fmt.Sprintf("@%s", paramName), params, true, nil
		case "prefix", "PREFIX":
			identSQL, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}

			constSQL, _, _, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = constSQL

			return fmt.Sprintf("STARTS_WITH(%s, @%s)", identSQL, paramName), params, false, nil
		case "suffix", "SUFFIX":
			identSQL, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}

			constSQL, _, _, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = constSQL

			return fmt.Sprintf("ENDS_WITH(%s, @%s)", identSQL, paramName), params, false, nil
		case "@in":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the IN operation is a registered identifier
			leftSQL = f.parseIdentifier(leftSQL.(string))

			rightSQL, _, _, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}
			return fmt.Sprintf("%s IN UNNEST(%s)", leftSQL, rightSQL), params, false, nil
		case "like", "LIKE":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}

			rightSQL, _, _, err := f.parseExpr(call.Args[1], state)
			if err != nil {
				return "", nil, false, err
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL

			return fmt.Sprintf("%s LIKE @%s", leftSQL, paramName), params, false, nil
		case "lower", "LOWER":
			sql, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}

			return fmt.Sprintf("LOWER(%s)", sql), params, false, nil
		case "upper", "UPPER":
			sql, _, _, err := f.parseExpr(call.Args[0], state)
			if err != nil {
				return "", nil, false, err
			}

			return fmt.Sprintf("UPPER(%s)", sql), params, false, nil
		case "concat", "CONCAT":
			return f.parseMultiArgFunction("CONCAT", call.Args, state)
		case "greatest", "GREATEST":
			return f.parseMultiArgFunction("GREATEST", call.Args, state)
		case "least", "LEAST":
			return f.parseMultiArgFunction("LEAST", call.Args, state)
		case "coalesce", "COALESCE":
			return f.parseMultiArgFunction("COALESCE", call.Args, state)
		case "ifnull", "IFNULL":
			if len(call.Args) != 2 {
				return "", nil, false, fmt.Errorf("IFNULL function requires exactly 2 arguments")
			}
			return f.parseMultiArgFunction("IFNULL", call.Args, state)
		default:
			return "", nil, false, fmt.Errorf("unsupported function: %s", call.Function)
		}
	case *expr.Expr_IdentExpr:
		return f.parseIdentifier(expression.GetIdentExpr().GetName()), params, false, nil
	case *expr.Expr_ConstExpr:
		constExpr := expression.GetConstExpr()
		parsedConstant, err := f.parseConstant(constExpr)
		if err != nil {
			return "", nil, false, ErrInvalidFilter{
				filter: constExpr.String(),
				err:    err,
			}
		}

		return parsedConstant, params, false, nil
	case *expr.Expr_SelectExpr:
		selectExpr := expression.GetSelectExpr()

		operandSQL, err := f.parseSelectExpr(selectExpr, state)
		if err != nil {
			return "", nil, false, err
		}

		return operandSQL, params, false, nil
	case *expr.Expr_ListExpr:
		listExpr := expression.GetListExpr()
		var sqlList []any
		for _, elem := range listExpr.Elements {
			elemSQL, _, _, err := f.parseExpr(elem, state)
			if err != nil {
				return "", nil, false, err
			}
			sqlList = append(sqlList, elemSQL)
		}
		paramName := fmt.Sprintf("p%d", len(params))
		params[paramName] = sqlList

		return fmt.Sprintf("@%s", paramName), params, false, nil
	case *expr.Expr_StructExpr:
		structExpr := expression.GetStructExpr()
		var fieldMap []string
		for _, entry := range structExpr.Entries {
			fieldSQL, _, _, err := f.parseExpr(entry.GetMapKey(), state)
			if err != nil {
				return "", nil, false, err
			}
			valueSQL, _, _, err := f.parseExpr(entry.GetValue(), state)
			if err != nil {
				return "", nil, false, err
			}

			fieldMap = append(fieldMap, fmt.Sprintf("%s = %s", fieldSQL, valueSQL))
		}
		return fmt.Sprintf("{ %s }", strings.Join(fieldMap, ", ")), params, false, nil
	case *expr.Expr_ComprehensionExpr:
		return "", nil, false, fmt.Errorf("comprehension expressions are not supported")
	default:
		return "", nil, false, fmt.Errorf("unsupported expression: %v", expression.GetExprKind())
	}

	return "", params, false, nil
}

// parseMultiArgFunction handles SQL functions that accept a variable number of arguments.
//
// This is used for: CONCAT, GREATEST, LEAST, COALESCE, IFNULL
//
// Parameters:
//   - fnName: The SQL function name (e.g., "CONCAT", "GREATEST")
//   - args: The CEL expression arguments to the function
//   - state: Per-call parse state (accumulated params + caller-supplied params)
//
// Returns the same values as parseExpr.
//
// The function parameterizes constant arguments (strings, numbers) for safety,
// while passing through column references and nested function calls directly.
//
// Example:
//
//	concat(first_name, ' ', last_name) -> CONCAT(first_name, @p0, last_name)
//	greatest(a, b, 10) -> GREATEST(a, b, @p0)
func (f *Parser) parseMultiArgFunction(fnName string, args []*expr.Expr, state *parseState) (any, map[string]any, bool, error) {
	params := state.params

	var sqlParts []string
	for _, arg := range args {
		argSQL, _, _, err := f.parseExpr(arg, state)
		if err != nil {
			return "", nil, false, err
		}

		switch arg.GetExprKind().(type) {
		case *expr.Expr_ConstExpr:
			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = argSQL
			sqlParts = append(sqlParts, fmt.Sprintf("@%s", paramName))
		default:
			sqlParts = append(sqlParts, argSQL.(string))
		}
	}

	return fmt.Sprintf("%s(%s)", fnName, strings.Join(sqlParts, ", ")), params, false, nil
}

// parseConstant extracts the Go value from a CEL constant expression.
//
// Supported constant types:
//   - StringValue: Returns string
//   - BoolValue: Returns bool
//   - Int64Value: Returns int64
//   - BytesValue: Returns base64-encoded string
//   - DoubleValue: Returns float64
//   - NullValue: Returns the string "NULL"
//
// Unsupported constant types (return error):
//   - DurationValue: Use duration() function instead
//   - TimestampValue: Use timestamp() function instead
//   - Uint64Value: Not supported by Spanner
func (f *Parser) parseConstant(constExpr *expr.Constant) (any, error) {
	switch constExpr.ConstantKind.(type) {
	case *expr.Constant_StringValue:
		return constExpr.GetStringValue(), nil
	case *expr.Constant_BoolValue:
		return constExpr.GetBoolValue(), nil
	case *expr.Constant_Int64Value:
		return constExpr.GetInt64Value(), nil
	case *expr.Constant_BytesValue:
		return base64.StdEncoding.EncodeToString(constExpr.GetBytesValue()), nil
	case *expr.Constant_DoubleValue:
		return constExpr.GetDoubleValue(), nil
	case *expr.Constant_DurationValue:
		return "", fmt.Errorf("duration constants are not supported")
	case *expr.Constant_NullValue:
		return "NULL", nil
	case *expr.Constant_TimestampValue:
		return "", fmt.Errorf("timestamp constants are not supported")
	case *expr.Constant_Uint64Value:
		return "", fmt.Errorf("uint64 constants are not supported")
	default:
		return "", fmt.Errorf("unsupported constant: %v", constExpr)
	}
}

// parseSelectExpr handles field selection (e.g., `message.field`) in CEL expressions.
//
// It recursively resolves nested field access to build dotted paths like:
//   - Proto.field -> "Proto.field"
//   - user.address.city -> "user.address.city"
//
// The operand can be another SelectExpr (for deep nesting) or an IdentExpr (base case).
func (f *Parser) parseSelectExpr(selectExpr *expr.Expr_Select, state *parseState) (string, error) {
	// Recursively resolve the operand (which could itself be a SelectExpr or IdentExpr)
	operandSQL, _, _, err := f.parseExpr(selectExpr.Operand, state)
	if err != nil {
		return "", err
	}

	// Concatenate the operand and field (e.g., `user.address.city`)
	return fmt.Sprintf("%s.%s", operandSQL, selectExpr.Field), nil
}

// parseIdentifier transforms an identifier based on its registered type.
//
// If the identifier is registered in the parser's identifiers map, it applies
// the appropriate SQL transformation:
//
//   - reservedIdentifier: Wraps in backticks -> `select`
//   - timestampIdentifier: Converts protobuf Timestamp to Spanner TIMESTAMP
//   - durationIdentifier: Converts protobuf Duration to seconds (float)
//   - dateIdentifier: Converts google.type.Date to Spanner DATE
//   - enumStringIdentifier: Casts to STRING -> CAST(field AS STRING)
//   - enumIntegerIdentifier: Casts to INT64 -> CAST(field AS INT64)
//
// If the identifier is not registered, it is returned unchanged.
func (f *Parser) parseIdentifier(sql string) string {
	if ident, ok := f.identifiers[sql]; ok {
		switch ident.(type) {
		case reservedIdentifier:
			sql = fmt.Sprintf("`%s`", sql)
		case timestampIdentifier:
			sql = fmt.Sprintf("TIMESTAMP_ADD(TIMESTAMP_SECONDS(%s.seconds),INTERVAL CAST(FLOOR(IFNULL(%s.nanos,0) / 1000) AS INT64) MICROSECOND)", sql, sql)
		case durationIdentifier:
			sql = fmt.Sprintf("(%s.seconds + IFNULL(%s.nanos,0) / 1e9)", sql, sql)
		case dateIdentifier:
			sql = fmt.Sprintf("DATE(%s.year, %s.month, %s.day)", sql, sql, sql)
		case enumStringIdentifier:
			sql = fmt.Sprintf("CAST(%s AS STRING)", sql)
		case enumIntegerIdentifier:
			sql = fmt.Sprintf("CAST(%s AS INT64)", sql)
		}
	}

	return sql
}
