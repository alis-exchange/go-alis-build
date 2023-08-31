package filtering

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/iancoleman/strcase"
	"go.alis.build/alog"

	"go.einride.tech/aip/filtering"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type DateEqual struct {
	LHS string `json:"lhs"`
	RHS string `json:"rhs"`
}

type FilterParser struct {
	filter       filtering.Filter
	params       map[string]interface{}
	paramCounter int
}

var relationalOperators = []string{"<", ">", "<=", ">=", "=", "!=", ":"}

// New instantiate a new FilterParser for the given filter
func New(filter filtering.Filter) *FilterParser {
	return &FilterParser{
		filter: filter,
		params: make(map[string]interface{}),
	}
}

// TranspileFilterToQuery takes in a filter and an initial query for the firestore collection that is being queried
// and extends the query with the filter
func TranspileFilterToQuery(filter filtering.Filter, query *firestore.Query) (*firestore.Query, error) {
	// Create a new filter parser
	var p *FilterParser

	// Initiate the parser
	p = New(filter)

	// Parse to get the params and resulting expression.
	expression, params, err := p.Parse()

	if err != nil {
		return nil, err
	}

	// Validate the item
	return p.Transpile(query, expression, params)
}

// Transpile takes in an AIP filter string and converts it to a firestore query
//
// expression is a map and takes the structure map[LHS:interface{} OP:string RHS:interface{}]
//
// Example:
//
//	map[string]interface{}{"LHS":"name" "OP":"=" "RHS":"param_0"}
//
// params is a map and takes the structure map[param_${index}: interface{}]
//
// Example:
//
//	map[string]interface{}{"param_0":"example name"}
func (p *FilterParser) Transpile(query *firestore.Query, expression interface{}, params map[string]interface{}) (*firestore.Query, error) {
	// TODO: Add support for traversal of nested objects,other custom types, etc.
	switch expression.(type) {
	case map[string]interface{}:
		// assert value to a map
		mapOfA, ok := expression.(map[string]interface{})

		if ok {
			lhs := mapOfA["LHS"]
			op := mapOfA["OP"]
			rhs := mapOfA["RHS"]

			// Check if op is in the list of relational Operators
			for _, o := range relationalOperators {
				if o == op {
					// Convert LHS (field name) to UpperCamelCase
					lhs = strcase.ToCamel(lhs.(string))
				}
			}

			switch op {
			case "=":
				{
					param := params[rhs.(string)]

					q := query.Where(lhs.(string), "==", param)

					return &q, nil
				}
			case "!=":
				{
					param := params[rhs.(string)]

					q := query.Where(lhs.(string), "!=", param)

					return &q, nil
				}
			case ">":
				{
					param := params[rhs.(string)]

					q := query.Where(lhs.(string), ">", param)

					return &q, nil
				}
			case ">=":
				{
					param := params[rhs.(string)]

					q := query.Where(lhs.(string), ">=", param)

					return &q, nil
				}
			case "<":
				{
					param := params[rhs.(string)]

					q := query.Where(lhs.(string), "<", param)

					return &q, nil
				}
			case "<=":
				{
					param := params[rhs.(string)]

					q := query.Where(lhs.(string), "<=", param)

					return &q, nil
				}
			case ":":
				{
					param := params[rhs.(string)]

					q := query.Where(lhs.(string), "in", strings.Split(param.(string), ","))

					return &q, nil
				}
			case "OR":
				return nil, errors.New("OR operator is not supported")
			case "AND":
				// Get the results for the left hand side expression
				lhsQuery, err := p.Transpile(query, lhs, params)
				if err != nil {
					return nil, err
				}

				// Get the results for the right hand side expression
				rhsQuery, err := p.Transpile(lhsQuery, rhs, params)
				if err != nil {
					return nil, err
				}

				return rhsQuery, nil
			case "NOT":
				return nil, errors.New("NOT operator is not supported")

			default:
				return nil, fmt.Errorf("unsupported operator/function %s", op.(string))
			}

		}
	default:
		return nil, fmt.Errorf("unsupported operator/function %s", expression.(string))
	}

	return nil, nil
}

func (p *FilterParser) Parse() (interface{}, map[string]interface{}, error) {
	if p.filter.CheckedExpr == nil {
		return true, nil, nil
	}

	resultExpr, err := p.parseExpr(p.filter.CheckedExpr.Expr)
	if err != nil {
		return nil, nil, err
	}

	params := p.params
	if p.paramCounter == 0 {
		params = nil
	}
	return resultExpr, params, nil
}

func (p *FilterParser) parseExpr(e *expr.Expr) (interface{}, error) {

	switch e.GetExprKind().(type) {
	case *expr.Expr_CallExpr:
		return p.parseCallExpr(e)
	case *expr.Expr_ConstExpr:
		return p.parseConstExpr(e)
	case *expr.Expr_IdentExpr:
		return p.parseIdentExpr(e)
	case *expr.Expr_SelectExpr:
		return p.parseSelectExpr(e)
	default:
		alog.Warnf(context.Background(), "unsupported expression type. Please implement this type")
		alog.Warnf(context.Background(), reflect.TypeOf(e.GetExprKind()).String())
		return nil, nil
	}
}

func (p *FilterParser) parseCallExpr(e *expr.Expr) (interface{}, error) {

	switch e.GetCallExpr().Function {
	case filtering.FunctionHas:
		return p.parseFunctionHas(e)
	case filtering.FunctionEquals:
		return p.parseFunctionComparison(e, filtering.FunctionEquals)
	case filtering.FunctionNotEquals:
		return p.parseFunctionComparison(e, filtering.FunctionNotEquals)
	case filtering.FunctionLessThan:
		return p.parseFunctionComparison(e, filtering.FunctionLessThan)
	case filtering.FunctionLessEquals:
		return p.parseFunctionComparison(e, filtering.FunctionLessEquals)
	case filtering.FunctionGreaterThan:
		return p.parseFunctionComparison(e, filtering.FunctionGreaterThan)
	case filtering.FunctionGreaterEquals:
		return p.parseFunctionComparison(e, filtering.FunctionGreaterEquals)
	case filtering.FunctionAnd:
		return p.parseFunctionLogical(e, filtering.FunctionAnd)
	case filtering.FunctionOr:
		return p.parseFunctionLogical(e, filtering.FunctionOr)
	case filtering.FunctionNot:
		return p.parseFunctionNot(e)
	case filtering.FunctionTimestamp:
		return p.parseFunctionTimestamp(e)
	case "date":
		return p.parseFunctionDate(e)
	case "dateEqual":
		return p.parseFunctionDateEqual(e)
	default:
		alog.Warnf(context.Background(), "unsupported function. Please implement this function")
		alog.Warnf(context.Background(), e.GetCallExpr().Function)
		return "", nil
	}
}

func (p *FilterParser) parseConstExpr(e *expr.Expr) (interface{}, error) {
	switch kind := e.GetConstExpr().ConstantKind.(type) {
	case *expr.Constant_BoolValue:
		return p.param(kind.BoolValue), nil
	case *expr.Constant_DoubleValue:
		return p.param(kind.DoubleValue), nil
	case *expr.Constant_Int64Value:
		return p.param(kind.Int64Value), nil
	case *expr.Constant_StringValue:
		return p.param(kind.StringValue), nil
	case *expr.Constant_Uint64Value:
		return p.param(kind.Uint64Value), nil
	default:
		return nil, nil
	}
}

func (p *FilterParser) parseIdentExpr(e *expr.Expr) (interface{}, error) {
	identExpr := e.GetIdentExpr()
	identType, ok := p.filter.CheckedExpr.TypeMap[e.Id]

	if !ok {
		return nil, fmt.Errorf("unknown type of ident expr %d", e.Id)
	}

	messageType := identType.GetMessageType()

	if messageType != "" {
		enumType, err := protoregistry.GlobalTypes.FindEnumByName(protoreflect.FullName(messageType))

		if err == nil {
			enumValue := enumType.Descriptor().Values().ByName(protoreflect.Name(identExpr.Name))

			if enumValue != nil {
				return p.param(enumValue.Number()), nil
			}
		}
	}

	return identExpr.Name, nil

}

func (p *FilterParser) parseSelectExpr(e *expr.Expr) (interface{}, error) {
	selectExpr := e.GetSelectExpr()
	operand, err := p.parseExpr(selectExpr.Operand)

	if err != nil {
		return nil, err
	}

	return operand, nil
}

func (p *FilterParser) parseFunctionHas(e *expr.Expr) (interface{}, error) {
	callExpr := e.GetCallExpr()

	if len(callExpr.Args) != 2 {
		return nil, fmt.Errorf("unexpected number of arguments to `in` expression: %d", len(callExpr.Args))
	}

	identExpr := callExpr.Args[0]
	constExpr := callExpr.Args[1]
	if identExpr.GetIdentExpr() == nil {
		return nil, fmt.Errorf("TODO: add support for transpiling `:` where LHS is other than Ident")
	}
	if constExpr.GetConstExpr() == nil {
		return nil, fmt.Errorf("TODO: add support for transpiling `:` where RHS is other than Const")
	}
	identType, ok := p.filter.CheckedExpr.TypeMap[callExpr.Args[0].Id]
	if !ok {
		return nil, fmt.Errorf("unknown type of ident expr %d", e.Id)
	}

	switch {
	// Repeated primitives:
	// > Repeated fields query to see if the repeated structure contains a matching element.
	case identType.GetListType().GetElemType().GetPrimitive() != expr.Type_PRIMITIVE_TYPE_UNSPECIFIED:
		iden, err := p.parseIdentExpr(identExpr)
		if err != nil {
			return nil, err
		}

		con, err := p.parseConstExpr(constExpr)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"OP":  ":",
			"LHS": iden,
			"RHS": con,
		}, nil

	default:
		return nil, fmt.Errorf("TODO: add support for transpiling `:` on other types than repeated primitives")
	}
}

func (p *FilterParser) parseFunctionComparison(e *expr.Expr, operand string) (interface{}, error) {
	callExpr := e.GetCallExpr()

	if len(callExpr.Args) != 2 {
		return nil, fmt.Errorf(
			"unexpected number of arguments to `%s`: %d",
			callExpr.GetFunction(),
			len(callExpr.Args),
		)
	}

	lhsExpr, err := p.parseExpr(callExpr.Args[0])
	if err != nil {
		return nil, err
	}

	rhsExpr, err := p.parseExpr(callExpr.Args[1])
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"OP":  operand,
		"LHS": lhsExpr,
		"RHS": rhsExpr,
	}, nil

}

// parseFunctionLogical get the left and right hand side values of filters that contain the operand AND or OR
func (p *FilterParser) parseFunctionLogical(e *expr.Expr, operand string) (interface{}, error) {
	callExpr := e.GetCallExpr()

	if len(callExpr.Args) != 2 {
		return nil, fmt.Errorf(
			"unexpected number of arguments to `%s`: %d",
			callExpr.GetFunction(),
			len(callExpr.Args),
		)
	}

	lhsExpr, err := p.parseExpr(callExpr.Args[0])
	if err != nil {
		return nil, err
	}
	rhsExpr, err := p.parseExpr(callExpr.Args[1])
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"OP":  operand,
		"LHS": lhsExpr,
		"RHS": rhsExpr,
	}, nil
}

// parseFunctionNot gets the right hand side value of filters that contain operand NOT
func (p *FilterParser) parseFunctionNot(e *expr.Expr) (interface{}, error) {
	callExpr := e.GetCallExpr()

	if len(callExpr.Args) != 1 {
		return nil, fmt.Errorf(
			"unexpected number of arguments to `%s` expression: %d",
			filtering.FunctionNot,
			len(callExpr.Args),
		)
	}
	rhsExpr, err := p.parseExpr(callExpr.Args[0])
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"OP":  "NOT",
		"RHS": rhsExpr,
	}, nil
}

// parseFunctionTimestamp gets the time stamp from filters that contain timestamp(${TIMESTAMP})
func (p *FilterParser) parseFunctionTimestamp(e *expr.Expr) (interface{}, error) {
	callExpr := e.GetCallExpr()
	if len(callExpr.Args) != 1 {
		return nil, fmt.Errorf(
			"unexpected number of arguments to `%s`: %d", callExpr.Function, len(callExpr.Args),
		)
	}

	constArg, ok := callExpr.Args[0].ExprKind.(*expr.Expr_ConstExpr)
	if !ok {
		return nil, fmt.Errorf("expected constant string arg to %s", callExpr.Function)
	}

	stringArg, ok := constArg.ConstExpr.ConstantKind.(*expr.Constant_StringValue)
	if !ok {
		return nil, fmt.Errorf("expected constant string arg to %s", callExpr.Function)
	}

	timeArg, err := time.Parse(time.RFC3339, stringArg.StringValue)
	if err != nil {
		return nil, fmt.Errorf("invalid string arg to %s: %w", callExpr.Function, err)
	}

	return p.param(timeArg), nil
}

func (p *FilterParser) parseFunctionDate(e *expr.Expr) (interface{}, error) {

	callExpr := e.GetCallExpr()
	if len(callExpr.Args) != 1 {
		return nil, fmt.Errorf(
			"unexpected number of arguments to `%s`: %d", callExpr.Function, len(callExpr.Args),
		)
	}

	constArg, ok := callExpr.Args[0].ExprKind.(*expr.Expr_ConstExpr)
	if !ok {
		return nil, fmt.Errorf("expected constant string arg to %s", callExpr.Function)
	}

	stringArg, ok := constArg.ConstExpr.ConstantKind.(*expr.Constant_StringValue)
	if !ok {
		return nil, fmt.Errorf("expected constant string arg to %s", callExpr.Function)
	}

	err := validateArgument("date", stringArg.StringValue, `^(\d{4}-\d{2}-\d{2})$`)
	if err != nil {
		return nil, err
	}

	d, err := parseISOStringToDate(stringArg.StringValue)
	if err != nil {
		return nil, err
	}

	return p.param(d), nil
}

func (p *FilterParser) parseFunctionDateEqual(e *expr.Expr) (interface{}, error) {

	callExpr := e.GetCallExpr()
	if len(callExpr.Args) != 2 {
		return nil, fmt.Errorf(
			"unexpected number of arguments to `%s`: %d", callExpr.Function, len(callExpr.Args),
		)
	}

	identArg, ok := callExpr.Args[0].ExprKind.(*expr.Expr_IdentExpr)
	if !ok {
		log.Println(fmt.Errorf("expected constant string arg to %s", callExpr.Function))
		return nil, fmt.Errorf("expected constant string arg to %s", callExpr.Function)
	}

	identArg2, ok := callExpr.Args[1].ExprKind.(*expr.Expr_IdentExpr)
	if !ok {
		log.Println(fmt.Errorf("expected constant string arg to %s", callExpr.Function))
		return nil, fmt.Errorf("expected constant string arg to %s", callExpr.Function)
	}

	arg1 := identArg.IdentExpr.GetName()
	arg2 := identArg2.IdentExpr.GetName()

	return &DateEqual{
		LHS: arg1,
		RHS: arg2,
	}, nil
}

func (p *FilterParser) param(param interface{}) string {
	par := p.nextParam()
	p.params[par] = param
	return par
}

func (p *FilterParser) nextParam() string {
	param := "param_" + strconv.Itoa(p.paramCounter)
	p.paramCounter++
	return param
}
