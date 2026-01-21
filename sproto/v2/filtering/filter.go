package filtering

import (
	"fmt"
	"regexp"

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

	logicalEqRegex, err := regexp.Compile(`\s+=\s+`)
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

// sanitize transforms a filter string from SQL-like syntax to CEL syntax.
//
// It performs the following transformations:
//   - AND -> && (logical AND)
//   - OR  -> || (logical OR)
//   - " = " -> " == " (equality operator, with spaces to avoid matching >=, <=, !=)
//   - NULL -> null (null literal)
//   - IN -> in (membership operator)
//
// This allows users to write filters using familiar SQL syntax while
// maintaining compatibility with the CEL parser.
func (f *Parser) sanitize(filter string) string {
	filter = f.sanitizersRegex.logicalAndRegex.ReplaceAllString(filter, "&&")
	filter = f.sanitizersRegex.logicalOrRegex.ReplaceAllString(filter, "||")
	filter = f.sanitizersRegex.logicalEqRegex.ReplaceAllString(filter, " == ")
	filter = f.sanitizersRegex.nullRegex.ReplaceAllString(filter, "null")
	filter = f.sanitizersRegex.inRegex.ReplaceAllString(filter, "in")

	return filter
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

May return an ErrInvalidFilter error if the filter is invalid.
*/
func (f *Parser) Parse(filter string) (*spanner.Statement, error) {
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

	sql, params, _, err := f.parseExpr(parsedExpr.GetExpr(), nil)
	if err != nil {
		return nil, ErrInvalidFilter{
			filter: filter,
			err:    err,
		}
	}

	// Convert params to their most concrete types
	for key, param := range params {
		params[key] = convertToConcreteType(param)
	}

	return &spanner.Statement{
		SQL:    sql.(string),
		Params: params,
	}, nil
}
