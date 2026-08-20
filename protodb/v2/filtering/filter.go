// Package filtering converts AIP-160 CEL filter expressions into Spanner statements.
package filtering

import (
	"fmt"
	"regexp"
	"strings"

	"cloud.google.com/go/spanner"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/parser"
)

// sanitizersRegex holds compiled regular expressions used to transform
// CEL syntax to a format compatible with the parser.
//
// These transformations convert SQL-like syntax (AND, OR, =, NULL, IN)
// to CEL syntax (&&, ||, ==, null, in) before parsing.
type sanitizersRegex struct {
	logicalAndRegex *regexp.Regexp // Matches word "AND" for conversion to "&&"
	logicalOrRegex  *regexp.Regexp // Matches word "OR" for conversion to "||"
	logicalEqRegex  *regexp.Regexp // Matches " = " for conversion to " == "
	nullRegex       *regexp.Regexp // Matches word "NULL" for conversion to "null"
	inRegex         *regexp.Regexp // Matches word "IN" for conversion to "in"
}

/*
Parser is a CEL filter expression to Spanner query parser.

It is used to parse a CEL filter expression and convert it to a Spanner statement.
*/
type Parser struct {
	identifiers     map[string]Identifier
	sanitizersRegex *sanitizersRegex
}

/*
NewParser creates a new Filter parser instance with the given identifiers.

Identifiers are used to declare common protocol buffer types for conversion.
Common identifiers are Timestamp, Duration, Date etc.
*/
func NewParser(identifiers ...Identifier) (*Parser, error) {
	// Create a CEL environment with the given identifiers.
	identifiersMap := make(map[string]Identifier)

	for _, i := range identifiers {
		identifiersMap[i.Path()] = i
	}

	logicalAndRegex, err := regexp.Compile(`\bAND\b`)
	if err != nil {
		return nil, err
	}

	logicalOrRegex, err := regexp.Compile(`\bOR\b`)
	if err != nil {
		return nil, err
	}

	// The second group also accepts end-of-segment ($) because sanitize
	// operates on literal-stripped segments: for input like `a='x'`, the
	// unquoted segment is `a=` — with nothing after `=` inside that segment,
	// a regex requiring a trailing character would silently fail to match.
	logicalEqRegex, err := regexp.Compile(`([^<>!=])\s*=\s*([^=]|$)`)
	if err != nil {
		return nil, err
	}

	nullRegex, err := regexp.Compile(`\bNULL\b`)
	if err != nil {
		return nil, err
	}

	inRegex, err := regexp.Compile(`\bIN\b`)
	if err != nil {
		return nil, err
	}

	return &Parser{
		identifiers: identifiersMap,
		sanitizersRegex: &sanitizersRegex{
			logicalAndRegex: logicalAndRegex,
			logicalOrRegex:  logicalOrRegex,
			logicalEqRegex:  logicalEqRegex,
			nullRegex:       nullRegex,
			inRegex:         inRegex,
		},
	}, nil
}

/*
DeclareIdentifier declares a new Identifier in the environment.

This is useful when you want to add a new identifier to the environment after creating the Filter instance.

May return an ErrInvalidIdentifier error if the identifier is invalid.
*/
func (f *Parser) DeclareIdentifier(identifier Identifier) error {
	if f == nil {
		return nil
	}

	if f.identifiers == nil {
		f.identifiers = make(map[string]Identifier)
	}

	f.identifiers[identifier.Path()] = identifier

	return nil
}

// splitLiterals splits filter into alternating unquoted/quoted segments.
// Segments at even indices are outside string literals; odd indices are the
// literals themselves (quotes included). Handles both ' and " delimiters and
// backslash escapes.
func splitLiterals(filter string) []string {
	var segs []string
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(filter); i++ {
		c := filter[i]
		if quote == 0 {
			if c == '\'' || c == '"' {
				segs = append(segs, cur.String())
				cur.Reset()
				quote = c
			}
			cur.WriteByte(c)
		} else {
			cur.WriteByte(c)
			if c == '\\' && i+1 < len(filter) {
				i++
				cur.WriteByte(filter[i])
				continue
			}
			if c == quote {
				segs = append(segs, cur.String())
				cur.Reset()
				quote = 0
			}
		}
	}
	segs = append(segs, cur.String())
	return segs
}

// sanitize transforms a filter string from SQL-like syntax to CEL syntax.
//
// It performs the following transformations on the portions of the filter
// that fall outside of quoted string literals:
//   - AND -> && (logical AND)
//   - OR  -> || (logical OR)
//   - =   -> == (equality operator, with or without surrounding spaces)
//   - NULL -> null (null literal)
//   - IN -> in (membership operator)
//
// The filter is first split into alternating unquoted/quoted segments via
// splitLiterals so that keywords and operators appearing inside string
// literals (e.g. "BRAND AND CO" or "a=b") are never rewritten.
//
// This allows users to write filters using familiar SQL syntax while
// maintaining compatibility with the CEL parser.
func (f *Parser) sanitize(filter string) string {
	segs := splitLiterals(filter)
	for i := 0; i < len(segs); i += 2 { // unquoted segments only
		s := segs[i]
		s = f.sanitizersRegex.logicalAndRegex.ReplaceAllString(s, "&&")
		s = f.sanitizersRegex.logicalOrRegex.ReplaceAllString(s, "||")
		s = f.sanitizersRegex.logicalEqRegex.ReplaceAllString(s, "$1 == $2")
		s = f.sanitizersRegex.nullRegex.ReplaceAllString(s, "null")
		s = f.sanitizersRegex.inRegex.ReplaceAllString(s, "in")
		segs[i] = s
	}
	return strings.Join(segs, "")
}

/*
Parse parses a CEL filter expression and returns a Spanner statement.

Examples:

	parser.Parse("Proto.effective_date.year > 2021 AND create_time > timestamp('2021-01-01T00:00:00Z') OR expire_after > duration('1h')")
	parser.Parse("key = 'resources/1' OR Proto.effective_date = date('2021-01-01')")
	parser.Parse("Proto.state = 'ACTIVE'"
	parser.Parse("key IN ['resources/1', 'resources/2']")
	parser.Parse("effective_date != null)
	parser.Parse("count >= 10)

Callers may bind named parameters referenced via param('name') in the filter
by passing a params map. Values in the map are bound directly as Spanner
query parameters (never spliced into SQL text); param('name') resolves to
params[0]["name"]. Referencing an unknown name, or using param(...) without
supplying a params map, returns an ErrInvalidFilter error.

May return an ErrInvalidFilter error if the filter is invalid.
*/
func (f *Parser) Parse(filter string, params ...map[string]any) (*spanner.Statement, error) {
	var callerParams map[string]any
	if len(params) > 0 {
		callerParams = params[0]
	}

	filter = f.sanitize(filter)

	source := common.NewTextSource(filter)
	p, err := parser.NewParser()
	if err != nil {
		return nil, ErrInvalidFilter{
			filter: filter,
			err:    err,
		}
	}

	parsed, errors := p.Parse(source)
	if errors != nil && len(errors.GetErrors()) > 0 {
		return nil, ErrInvalidFilter{
			filter: filter,
			err:    fmt.Errorf("%s", errors.ToDisplayString()),
		}
	}

	// Convert AST to protobuf format
	parsedExpr, err := ast.ToProto(parsed)
	if err != nil {
		return nil, ErrInvalidFilter{
			filter: filter,
			err:    err,
		}
	}

	sql, stmtParams, _, err := f.parseExpr(parsedExpr.GetExpr(), &parseState{callerParams: callerParams})
	if err != nil {
		return nil, ErrInvalidFilter{
			filter: filter,
			err:    err,
		}
	}

	// Convert params to their most concrete types. This is a no-op (identity)
	// for scalar passthrough values such as strings, numbers, and time.Time
	// (they hit convertValue's default case), so caller-bound param('name')
	// values are unaffected by this pass.
	for key, param := range stmtParams {
		stmtParams[key] = convertToConcreteType(param)
	}

	return &spanner.Statement{
		SQL:    sql.(string),
		Params: stmtParams,
	}, nil
}
