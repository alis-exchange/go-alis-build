package filtering

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// parseExpr converts a CEL expression into SQL string and parameters
func (f *Parser) parseExpr(expression *expr.Expr, params map[string]any) (string, map[string]any, bool, error) {
	if params == nil {
		params = make(map[string]any)
	}

	switch expression.GetExprKind().(type) {
	case *expr.Expr_CallExpr:
		call := expression.GetCallExpr()

		switch call.Function {
		case "_&&_":
			leftSQL, leftParams, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, rightParams, _, err := f.parseExpr(call.Args[1], params)
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
			leftSQL, leftParams, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, rightParams, _, err := f.parseExpr(call.Args[1], params)
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
			leftSQL, _, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], params)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL)

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s > %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			return fmt.Sprintf("%s > @%s", leftSQL, paramName), params, false, nil
		case "_>=_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], params)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL)

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s >= %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			return fmt.Sprintf("%s >= @%s", leftSQL, paramName), params, false, nil

		case "_<_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], params)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL)

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s < %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			return fmt.Sprintf("%s < @%s", leftSQL, paramName), params, false, nil
		case "_<=_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], params)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL)

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s <= %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			return fmt.Sprintf("%s <= @%s", leftSQL, paramName), params, false, nil
		case "_==_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], params)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL)

			// Check if the right side of the comparison is a function.
			// If it is, we don't need to add it as a parameter but instead as a literal value
			if isFunction {
				return fmt.Sprintf("%s = %s", leftSQL, rightSQL), params, false, nil
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = rightSQL
			return fmt.Sprintf("%s = @%s", leftSQL, paramName), params, false, nil
		case "_!=_":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, isFunction, err := f.parseExpr(call.Args[1], params)
			if err != nil {
				return "", nil, false, err
			}

			// Check if the left side of the comparison is a registered identifier
			// and apply the necessary transformation
			leftSQL = f.parseIdentifier(leftSQL)

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
		case "prefix", "PREFIX":
			identSQL, _, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}

			constSQL, _, _, err := f.parseExpr(call.Args[1], params)
			if err != nil {
				return "", nil, false, err
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = constSQL

			return fmt.Sprintf("STARTS_WITH(%s, @%s)", identSQL, paramName), params, false, nil
		case "suffix", "SUFFIX":
			identSQL, _, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}

			constSQL, _, _, err := f.parseExpr(call.Args[1], params)
			if err != nil {
				return "", nil, false, err
			}

			paramName := fmt.Sprintf("p%d", len(params))
			params[paramName] = constSQL

			return fmt.Sprintf("ENDS_WITH(%s, @%s)", identSQL, paramName), params, false, nil
		case "@in":
			leftSQL, _, _, err := f.parseExpr(call.Args[0], params)
			if err != nil {
				return "", nil, false, err
			}
			rightSQL, _, _, err := f.parseExpr(call.Args[1], params)
			if err != nil {
				return "", nil, false, err
			}
			return fmt.Sprintf("%s IN UNNEST(%s)", leftSQL, rightSQL), params, false, nil

		default:
			return "", nil, false, fmt.Errorf("unsupported function: %s", call.Function)
		}
	case *expr.Expr_IdentExpr:
		return f.parseIdentifier(expression.GetIdentExpr().GetName()), params, false, nil
	case *expr.Expr_ConstExpr:
		constExpr := expression.GetConstExpr()
		switch constExpr.ConstantKind.(type) {
		case *expr.Constant_StringValue:
			return fmt.Sprintf("%s", constExpr.GetStringValue()), params, false, nil
		case *expr.Constant_BoolValue:
			return fmt.Sprintf("%t", constExpr.GetBoolValue()), params, false, nil
		case *expr.Constant_Int64Value:
			return fmt.Sprintf("%d", constExpr.GetInt64Value()), params, false, nil
		case *expr.Constant_BytesValue:
			return base64.StdEncoding.EncodeToString(constExpr.GetBytesValue()), params, false, nil
		case *expr.Constant_DoubleValue:
			return fmt.Sprintf("%f", constExpr.GetDoubleValue()), params, false, nil
		case *expr.Constant_DurationValue:
			return "", params, false, fmt.Errorf("duration constants are not supported")
		case *expr.Constant_NullValue:
			return "NULL", params, false, nil
		case *expr.Constant_TimestampValue:
			return "", params, false, fmt.Errorf("timestamp constants are not supported")
		case *expr.Constant_Uint64Value:
			return "", params, false, fmt.Errorf("uint64 constants are not supported")
		default:
			return "", nil, false, fmt.Errorf("unsupported constant: %v", constExpr)
		}
	case *expr.Expr_SelectExpr:
		selectExpr := expression.GetSelectExpr()

		operandSQL, err := f.parseSelectExpr(selectExpr, params)
		if err != nil {
			return "", nil, false, err
		}

		return operandSQL, params, false, nil
	case *expr.Expr_ListExpr:
		listExpr := expression.GetListExpr()
		var sqlList []string
		for _, elem := range listExpr.Elements {
			elemSQL, _, _, err := f.parseExpr(elem, params)
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
			fieldSQL, _, _, err := f.parseExpr(entry.GetMapKey(), params)
			if err != nil {
				return "", nil, false, err
			}
			valueSQL, _, _, err := f.parseExpr(entry.GetValue(), params)
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

// parseSelectExpr handles field selection (e.g. `message.field`) in CEL expressions
func (f *Parser) parseSelectExpr(selectExpr *expr.Expr_Select, params map[string]interface{}) (string, error) {
	// Recursively resolve the operand (which could itself be a SelectExpr or IdentExpr)
	operandSQL, _, _, err := f.parseExpr(selectExpr.Operand, params)
	if err != nil {
		return "", err
	}

	// Concatenate the operand and field (e.g., `user.address.city`)
	return fmt.Sprintf("%s.%s", operandSQL, selectExpr.Field), nil
}

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
		}
	}

	return sql
}
