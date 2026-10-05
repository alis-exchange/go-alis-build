package filtering

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"

	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Predicate is a compiled filter that evaluates rows in memory, with the same
// meaning Parse gives the filter in Spanner SQL. Create one with
// Parser.Compile. A Predicate is safe for concurrent use.
type Predicate struct {
	parser *Parser
	root   *expr.Expr
	params map[string]any
}

// Compile parses filter with the same rules as Parse and returns a Predicate
// that evaluates it in memory, for adapters (such as an in-memory test table)
// that cannot run SQL. Any filter Parse rejects, Compile rejects with the same
// ErrInvalidFilter; params bind param('name') exactly as in Parse.
func (f *Parser) Compile(filter string, params ...map[string]any) (pred *Predicate, err error) {
	// Some filters Parse cannot render (a constant on the left of a
	// comparison, a bare literal) panic inside it; Compile reports them as
	// invalid instead of taking the caller down.
	defer func() {
		if r := recover(); r != nil {
			pred, err = nil, ErrInvalidFilter{filter: filter, err: fmt.Errorf("unsupported filter: %v", r)}
		}
	}()
	// Parse validates everything the SQL path checks (operators, arity,
	// param() names, null in ordering comparisons), so both paths accept
	// exactly the same filters and fail with the same errors.
	if _, err := f.Parse(filter, params...); err != nil {
		return nil, err
	}
	root, _, err := f.parseFilter(filter)
	if err != nil {
		return nil, err
	}
	var callerParams map[string]any
	if len(params) > 0 {
		callerParams = params[0]
	}
	return &Predicate{parser: f, root: root, params: callerParams}, nil
}

// Match reports whether the row described by resolve satisfies the filter.
//
// resolve returns the raw value at an identifier path such as "key" or
// "Book.create_time": nil for NULL, a Go scalar (string, int64, float64,
// bool, []byte), a time.Time, a proto.Message, or a
// protoreflect.EnumValueDescriptor for an enum field.
//
// Evaluation follows SQL three-valued logic: a comparison with NULL is
// unknown, and only rows for which the filter is true match. Constructs the
// in-memory evaluator does not support return a codes.Unimplemented status;
// comparing values of incompatible types returns codes.InvalidArgument.
func (p *Predicate) Match(resolve func(path string) (any, error)) (bool, error) {
	v, err := p.evalBool(p.root, resolve)
	if err != nil {
		return false, err
	}
	return v == true, nil
}

// eval evaluates e to a value. Booleans are bool, and NULL (including an
// unknown truth value) is nil.
func (p *Predicate) eval(e *expr.Expr, resolve func(string) (any, error)) (any, error) {
	switch kind := e.GetExprKind().(type) {
	case *expr.Expr_ConstExpr:
		return constValue(kind.ConstExpr)
	case *expr.Expr_IdentExpr, *expr.Expr_SelectExpr:
		path, err := selectPath(e)
		if err != nil {
			return nil, err
		}
		raw, err := resolve(path)
		if err != nil {
			return nil, err
		}
		return p.normalize(path, raw)
	case *expr.Expr_CallExpr:
		return p.evalCall(kind.CallExpr, resolve)
	case *expr.Expr_ListExpr:
		return nil, unimplemented("a list outside IN")
	case *expr.Expr_StructExpr:
		return nil, unimplemented("struct and map literals")
	case *expr.Expr_ComprehensionExpr:
		return nil, unimplemented("comprehensions")
	default:
		return nil, unimplemented(fmt.Sprintf("expression %T", kind))
	}
}

func (p *Predicate) evalCall(call *expr.Expr_Call, resolve func(string) (any, error)) (any, error) {
	switch call.GetFunction() {
	case "_&&_", "_||_":
		left, err := p.evalBool(call.GetArgs()[0], resolve)
		if err != nil {
			return nil, err
		}
		right, err := p.evalBool(call.GetArgs()[1], resolve)
		if err != nil {
			return nil, err
		}
		if call.GetFunction() == "_&&_" {
			return and(left, right), nil
		}
		return or(left, right), nil
	case "_==_", "_!=_":
		not := call.GetFunction() == "_!=_"
		left, right := call.GetArgs()[0], call.GetArgs()[1]
		if isNullConst(left) || isNullConst(right) {
			operand := left
			if isNullConst(left) {
				operand = right
			}
			v, err := p.eval(operand, resolve)
			if err != nil {
				return nil, err
			}
			return (v == nil) != not, nil
		}
		// NaN equals nothing, so != against NaN is true.
		return p.evalCompare(left, right, resolve, not, func(c int) bool { return (c == 0) != not })
	case "_<_":
		return p.evalCompare(call.GetArgs()[0], call.GetArgs()[1], resolve, false, func(c int) bool { return c < 0 })
	case "_<=_":
		return p.evalCompare(call.GetArgs()[0], call.GetArgs()[1], resolve, false, func(c int) bool { return c <= 0 })
	case "_>_":
		return p.evalCompare(call.GetArgs()[0], call.GetArgs()[1], resolve, false, func(c int) bool { return c > 0 })
	case "_>=_":
		return p.evalCompare(call.GetArgs()[0], call.GetArgs()[1], resolve, false, func(c int) bool { return c >= 0 })
	case "@in":
		return p.evalIn(call.GetArgs()[0], call.GetArgs()[1], resolve)
	default:
		return p.evalFunction(call, resolve)
	}
}

// evalFunction evaluates the named functions Parse supports. Names match in
// lower or upper case, as in Parse.
func (p *Predicate) evalFunction(call *expr.Expr_Call, resolve func(string) (any, error)) (any, error) {
	args := call.GetArgs()
	name := strings.ToLower(call.GetFunction())
	if err := checkArity(name, len(args)); err != nil {
		return nil, err
	}
	switch name {
	case "timestamp":
		t, err := time.Parse(time.RFC3339Nano, args[0].GetConstExpr().GetStringValue())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "filtering: timestamp(): %v", err)
		}
		return t, nil
	case "duration":
		// Parse binds durations as float seconds, matching the Duration
		// identifier's SQL rewrite.
		d, err := time.ParseDuration(args[0].GetConstExpr().GetStringValue())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "filtering: duration(): %v", err)
		}
		return d.Seconds(), nil
	case "date":
		t, err := time.Parse(time.DateOnly, args[0].GetConstExpr().GetStringValue())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "filtering: date(): %v", err)
		}
		return t, nil
	case "param":
		// Compile already checked through Parse that the name is bound.
		return p.normalize("", p.params[args[0].GetConstExpr().GetStringValue()])
	}

	values, err := p.evalArgs(name, args, resolve)
	if err != nil {
		return nil, err
	}
	switch name {
	case "coalesce", "ifnull":
		for _, v := range values {
			if v != nil {
				return v, nil
			}
		}
		return nil, nil
	}
	for _, v := range values {
		if v == nil {
			return nil, nil // every other function returns NULL on a NULL input
		}
	}
	switch name {
	case "prefix", "suffix", "like":
		s, err := stringArg(name, values[0])
		if err != nil {
			return nil, err
		}
		pattern, err := stringArg(name, values[1])
		if err != nil {
			return nil, err
		}
		switch name {
		case "prefix":
			return strings.HasPrefix(s, pattern), nil
		case "suffix":
			return strings.HasSuffix(s, pattern), nil
		default:
			return likeMatch(s, pattern)
		}
	case "lower", "upper":
		s, err := stringArg(name, values[0])
		if err != nil {
			return nil, err
		}
		if name == "lower" {
			return strings.ToLower(s), nil
		}
		return strings.ToUpper(s), nil
	case "concat":
		var b strings.Builder
		for _, v := range values {
			s, err := stringArg(name, v)
			if err != nil {
				return nil, err
			}
			b.WriteString(s)
		}
		return b.String(), nil
	case "greatest", "least":
		for _, v := range values {
			if isNaN(v) {
				return math.NaN(), nil // Spanner: any NaN input makes the result NaN
			}
		}
		best := values[0]
		for _, v := range values[1:] {
			c, err := compare(v, best)
			if err != nil {
				return nil, err
			}
			if (name == "greatest" && c > 0) || (name == "least" && c < 0) {
				best = v
			}
		}
		return best, nil
	default:
		return nil, unimplemented("function " + call.GetFunction())
	}
}

// singleArgRendered names the functions whose first argument Parse renders
// as raw SQL text rather than binding it as a parameter.
var singleArgRendered = map[string]bool{"prefix": true, "suffix": true, "like": true, "lower": true, "upper": true}

// checkArity rejects calls Parse lets through with an argument count the
// function cannot take.
func checkArity(name string, n int) error {
	want := map[string]int{
		"timestamp": 1, "duration": 1, "date": 1, "param": 1,
		"prefix": 2, "suffix": 2, "like": 2, "lower": 1, "upper": 1, "ifnull": 2,
	}
	if w, ok := want[name]; ok && n != w {
		return status.Errorf(codes.InvalidArgument, "filtering: %s() takes %d argument(s), got %d", name, w, n)
	}
	if n == 0 {
		return status.Errorf(codes.InvalidArgument, "filtering: %s() needs at least one argument", name)
	}
	return nil
}

// boundAsText reports whether Parse binds e, when it is a comparison's
// right-hand side or a pattern, as the literal text of its SQL rendering.
// Parse embeds only timestamp(), duration(), date() and param() as SQL; any
// other call there becomes a string parameter such as "CONCAT(@p0, @p1)".
func boundAsText(e *expr.Expr) bool {
	call := e.GetCallExpr()
	if call == nil {
		return false
	}
	switch strings.ToLower(call.GetFunction()) {
	case "timestamp", "duration", "date", "param":
		return false
	default:
		return true
	}
}

// evalArgs evaluates a function's arguments, rejecting the forms where Parse
// renders something other than the obvious reading: a null constant (bound
// as the string "NULL") and, for prefix/suffix/like, an identifier as the
// pattern (bound as a string literal).
func (p *Predicate) evalArgs(name string, args []*expr.Expr, resolve func(string) (any, error)) ([]any, error) {
	values := make([]any, len(args))
	for i, arg := range args {
		if isNullConst(arg) {
			return nil, unimplemented("null as a function argument")
		}
		pattern := i == 1 && (name == "prefix" || name == "suffix" || name == "like")
		if pattern && (isPath(arg) || boundAsText(arg)) {
			return nil, unimplemented("an identifier or computed value as the " + name + " pattern")
		}
		if i == 0 && arg.GetConstExpr() != nil && singleArgRendered[name] {
			// Parse renders this argument unquoted, so Spanner reads a
			// string constant here as a column name.
			return nil, unimplemented("a constant as the first argument of " + name)
		}
		v, err := p.eval(arg, resolve)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return values, nil
}

func stringArg(function string, v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", status.Errorf(codes.InvalidArgument, "filtering: %s() needs a string, got %T", function, v)
	}
	return s, nil
}

// likeMatch implements SQL LIKE: % matches any run of characters, _ matches
// exactly one, \ makes the next character literal, and the match is
// case-sensitive. A pattern ending in a lone \ is invalid.
func likeMatch(s, pattern string) (bool, error) {
	type token struct {
		r       rune
		literal bool
	}
	var tokens []token
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' {
			if i+1 == len(runes) {
				return false, status.Error(codes.InvalidArgument, "filtering: LIKE pattern ends with an escape character")
			}
			i++
			tokens = append(tokens, token{r: runes[i], literal: true})
			continue
		}
		tokens = append(tokens, token{r: runes[i]})
	}

	// Iterative wildcard match with backtracking to the last %.
	text := []rune(s)
	ti, pi := 0, 0
	star, mark := -1, 0
	for ti < len(text) {
		switch {
		case pi < len(tokens) && !tokens[pi].literal && tokens[pi].r == '%':
			star, mark = pi, ti
			pi++
		case pi < len(tokens) && ((!tokens[pi].literal && tokens[pi].r == '_') || tokens[pi].r == text[ti]):
			ti++
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			ti = mark
		default:
			return false, nil
		}
	}
	for pi < len(tokens) && !tokens[pi].literal && tokens[pi].r == '%' {
		pi++
	}
	return pi == len(tokens), nil
}

// evalBool evaluates e as a truth value: true, false, or nil for unknown.
func (p *Predicate) evalBool(e *expr.Expr, resolve func(string) (any, error)) (any, error) {
	v, err := p.eval(e, resolve)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	if _, ok := v.(bool); !ok {
		return nil, status.Errorf(codes.InvalidArgument, "filtering: %v is not a boolean condition", v)
	}
	return v, nil
}

// evalCompare evaluates both sides and applies test to their ordering. A NULL
// on either side makes the comparison unknown (nil), as in SQL; a NaN on
// either side makes it onNaN, following IEEE 754.
func (p *Predicate) evalCompare(left, right *expr.Expr, resolve func(string) (any, error), onNaN bool, test func(int) bool) (any, error) {
	if isPath(right) {
		// Parse binds a right-hand identifier as a string literal rather
		// than reading the column, so evaluating it as a column would
		// silently disagree with Spanner.
		return nil, unimplemented("an identifier on the right-hand side of a comparison")
	}
	if boundAsText(right) {
		return nil, unimplemented("a computed value on the right-hand side of a comparison")
	}
	a, err := p.eval(left, resolve)
	if err != nil {
		return nil, err
	}
	b, err := p.eval(right, resolve)
	if err != nil {
		return nil, err
	}
	if a == nil || b == nil {
		return nil, nil
	}
	// compare checks the types first, so NaN never hides a mismatch.
	c, err := compare(a, b)
	if err != nil {
		return nil, err
	}
	if isNaN(a) || isNaN(b) {
		return onNaN, nil
	}
	return test(c), nil
}

// evalIn implements `x IN [a, b, ...]`: true when x equals an element, NULL
// when x is NULL, and false otherwise.
func (p *Predicate) evalIn(left, list *expr.Expr, resolve func(string) (any, error)) (any, error) {
	if list.GetListExpr() == nil {
		return nil, unimplemented("IN with a right-hand side that is not a list literal")
	}
	elems := list.GetListExpr().GetElements()
	for _, el := range elems {
		if isNullConst(el) {
			// Parse binds a null list element as the string "NULL".
			return nil, unimplemented("null inside an IN list")
		}
		if isPath(el) {
			// Parse binds an identifier list element as a string literal.
			return nil, unimplemented("an identifier inside an IN list")
		}
		if boundAsText(el) {
			return nil, unimplemented("a computed value inside an IN list")
		}
	}
	v, err := p.eval(left, resolve)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	for _, el := range elems {
		ev, err := p.eval(el, resolve)
		if err != nil {
			return nil, err
		}
		c, err := compare(v, ev)
		if err != nil {
			return nil, err
		}
		if isNaN(v) || isNaN(ev) {
			continue
		}
		if c == 0 {
			return true, nil
		}
	}
	return false, nil
}

// normalize converts a resolved value into the evaluator's value model.
func (p *Predicate) normalize(_ string, raw any) (any, error) {
	switch v := raw.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case float32:
		return float64(v), nil
	default:
		return raw, nil
	}
}

// and applies SQL's three-valued AND to true, false and nil (unknown).
func and(a, b any) any {
	if a == false || b == false {
		return false
	}
	if a == nil || b == nil {
		return nil
	}
	return true
}

// or applies SQL's three-valued OR to true, false and nil (unknown).
func or(a, b any) any {
	if a == true || b == true {
		return true
	}
	if a == nil || b == nil {
		return nil
	}
	return false
}

// compare orders two non-NULL values of compatible types. int64 and float64
// compare numerically with each other, as Spanner coerces them.
func compare(a, b any) (int, error) {
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return strings.Compare(av, bv), nil
		}
	case int64:
		switch bv := b.(type) {
		case int64:
			return cmpOrdered(av, bv), nil
		case float64:
			return cmpOrdered(float64(av), bv), nil
		}
	case float64:
		switch bv := b.(type) {
		case float64:
			return cmpOrdered(av, bv), nil
		case int64:
			return cmpOrdered(av, float64(bv)), nil
		}
	case bool:
		if bv, ok := b.(bool); ok {
			switch {
			case av == bv:
				return 0, nil
			case bv:
				return -1, nil
			default:
				return 1, nil
			}
		}
	case time.Time:
		if bv, ok := b.(time.Time); ok {
			return av.Compare(bv), nil
		}
	case []byte:
		if bv, ok := b.([]byte); ok {
			return bytes.Compare(av, bv), nil
		}
	}
	return 0, status.Errorf(codes.InvalidArgument, "filtering: cannot compare %T with %T", a, b)
}

func cmpOrdered[T int64 | float64](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// constValue converts a CEL constant into the evaluator's value model. Unlike
// parseConstant it keeps bytes raw and null as nil.
func constValue(c *expr.Constant) (any, error) {
	switch kind := c.GetConstantKind().(type) {
	case *expr.Constant_NullValue:
		return nil, nil
	case *expr.Constant_StringValue:
		return kind.StringValue, nil
	case *expr.Constant_BoolValue:
		return kind.BoolValue, nil
	case *expr.Constant_Int64Value:
		return kind.Int64Value, nil
	case *expr.Constant_DoubleValue:
		return kind.DoubleValue, nil
	case *expr.Constant_BytesValue:
		return kind.BytesValue, nil
	default:
		return nil, unimplemented(fmt.Sprintf("constant %T", kind))
	}
}

// selectPath builds the dotted identifier path for an ident or select
// expression, exactly as parseSelectExpr renders it for SQL.
func selectPath(e *expr.Expr) (string, error) {
	switch kind := e.GetExprKind().(type) {
	case *expr.Expr_IdentExpr:
		return kind.IdentExpr.GetName(), nil
	case *expr.Expr_SelectExpr:
		operand, err := selectPath(kind.SelectExpr.GetOperand())
		if err != nil {
			return "", err
		}
		return operand + "." + kind.SelectExpr.GetField(), nil
	default:
		return "", unimplemented("field selection on a computed value")
	}
}

func unimplemented(construct string) error {
	return status.Errorf(codes.Unimplemented, "filtering: %s is not supported by in-memory evaluation", construct)
}

// isPath reports whether e is an identifier or a field selection.
func isPath(e *expr.Expr) bool {
	return e.GetIdentExpr() != nil || e.GetSelectExpr() != nil
}

func isNaN(v any) bool {
	f, ok := v.(float64)
	return ok && math.IsNaN(f)
}
