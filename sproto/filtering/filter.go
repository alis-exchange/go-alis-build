package filtering

import (
	"regexp"

	"cloud.google.com/go/spanner"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type sanitizersRegex struct {
	logicalAndRegex *regexp.Regexp
	logicalOrRegex  *regexp.Regexp
	logicalEqRegex  *regexp.Regexp
	nullRegex       *regexp.Regexp
	inRegex         *regexp.Regexp
}

/*
Parser is a CEL filter expression to Spanner query parser.

It is used to parse a CEL filter expression and convert it to a Spanner statement.
*/
type Parser struct {
	identifiers     map[string]Identifier
	env             *cel.Env
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
	var opts []cel.EnvOption
	for _, i := range identifiers {
		opts = append(opts, cel.Variable(i.Path(), i.envType()))
		identifiersMap[i.Path()] = i
	}
	opts = append(opts, cel.Types(&durationpb.Duration{}, &timestamppb.Timestamp{}, &date.Date{}, &money.Money{}), ext.Protos())

	env, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, err
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
		env:         env,
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
	env, err := f.env.Extend(cel.Variable(identifier.Path(), identifier.envType()))
	if err != nil {
		return ErrInvalidIdentifier{
			identifier: identifier.Path(),
			err:        err,
		}
	}

	f.env = env

	return nil
}

func (f *Parser) sanitize(filter string) string {
	filter = f.sanitizersRegex.logicalAndRegex.ReplaceAllString(filter, "&&")
	filter = f.sanitizersRegex.logicalOrRegex.ReplaceAllString(filter, "||")
	filter = f.sanitizersRegex.logicalEqRegex.ReplaceAllString(filter, " == ")
	filter = f.sanitizersRegex.nullRegex.ReplaceAllString(filter, "null")
	filter = f.sanitizersRegex.inRegex.ReplaceAllString(filter, "in")

	//filter = strings.ReplaceAll(filter, " TIMESTAMP(", " timestamp(")
	//filter = strings.ReplaceAll(filter, " DURATION(", " duration(")

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

	ast, issues := f.env.Parse(filter)
	if issues != nil && issues.Err() != nil {
		return nil, ErrInvalidFilter{
			filter: filter,
			err:    issues.Err(),
		}
	}

	expr, err := cel.AstToParsedExpr(ast)
	if err != nil {
		return nil, ErrInvalidFilter{
			filter: filter,
			err:    err,
		}
	}

	sql, params, _, err := f.parseExpr(expr.GetExpr(), nil)
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
