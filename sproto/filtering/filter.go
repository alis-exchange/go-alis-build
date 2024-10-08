package filtering

import (
	"regexp"

	"cloud.google.com/go/spanner"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/ext"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

/*
Identifier represents a CEL type identifier for certain types.

Commonly used for identifying and transforming protocol buffer types.
For example:
  - Timestamp() will convert a google.protobuf.Timestamp to a spanner Timestamp type.
  - Duration() will convert a google.protobuf.Duration to a spanner String type.
*/
type Identifier interface {
	Path() string
	envType() *cel.Type
}

type anyIdentifier struct {
	path string
}

func (t anyIdentifier) envType() *cel.Type {
	return cel.AnyType
}
func (t anyIdentifier) Path() string {
	return t.path
}

type boolIdentifier struct {
	path string
}

func (t boolIdentifier) envType() *cel.Type {
	return cel.BoolType
}
func (t boolIdentifier) Path() string {
	return t.path
}

type bytesIdentifier struct {
	path string
}

func (t bytesIdentifier) envType() *cel.Type {
	return cel.BytesType
}
func (t bytesIdentifier) Path() string {
	return t.path
}

type doubleIdentifier struct {
	path string
}

func (t doubleIdentifier) envType() *cel.Type {
	return cel.DoubleType
}
func (t doubleIdentifier) Path() string {
	return t.path
}

type durationIdentifier struct {
	path string
}

func (t durationIdentifier) envType() *cel.Type {
	return cel.DurationType
}
func (t durationIdentifier) Path() string {
	return t.path
}

type intIdentifier struct {
	path string
}

func (t intIdentifier) envType() *cel.Type {
	return cel.IntType
}
func (t intIdentifier) Path() string {
	return t.path
}

type nullIdentifier struct {
	path string
}

func (t nullIdentifier) envType() *cel.Type {
	return cel.NullType
}
func (t nullIdentifier) Path() string {
	return t.path
}

type stringIdentifier struct {
	path string
}

func (t stringIdentifier) envType() *cel.Type {
	return cel.StringType
}
func (t stringIdentifier) Path() string {
	return t.path
}

type timestampIdentifier struct {
	path string
}

func (t timestampIdentifier) envType() *cel.Type {
	return cel.TimestampType
}
func (t timestampIdentifier) Path() string {
	return t.path
}

type dateIdentifier struct {
	path string
}

func (t dateIdentifier) envType() *cel.Type {
	return cel.ObjectType("google.type.Date", traits.AdderType|
		traits.ComparerType|
		traits.NegatorType|
		traits.ReceiverType|
		traits.SubtractorType)
}
func (t dateIdentifier) Path() string {
	return t.path
}

type uintIdentifier struct {
	path string
}

func (t uintIdentifier) envType() *cel.Type {
	return cel.UintType
}
func (t uintIdentifier) Path() string {
	return t.path
}

type listIdentifier struct {
	path     string
	elemType *cel.Type
}

func (t listIdentifier) envType() *cel.Type {
	return cel.ListType(t.elemType)
}
func (t listIdentifier) Path() string {
	return t.path
}

type mapIdentifier struct {
	path      string
	keyType   *cel.Type
	valueType *cel.Type
}

func (t mapIdentifier) envType() *cel.Type {
	return cel.MapType(t.keyType, t.valueType)
}
func (t mapIdentifier) Path() string {
	return t.path
}

/*
Duration enables conversion of google.protobuf.Duration to a spanner int type.

It takes in the path to the column/field.

Example:

	Duration("expire_after")
	Duration("Proto.expire_after")
*/
func Duration(path string) Identifier {
	return durationIdentifier{
		path: path,
	}
}

/*
Timestamp enables conversion of google.protobuf.Timestamp to a spanner String/Timestamp type.

It takes in the path to the column/field.

Example:

	Timestamp("create_time")
	Timestamp("Proto.create_time")
*/
func Timestamp(path string) Identifier {
	return timestampIdentifier{
		path: path,
	}
}

/*
Date enables conversion of google.type.Date to a spanner String/Date type.

It takes in the path to the column/field.

Example:

	Date("effective_date")
	Date("Proto.effective_date")
*/
func Date(path string) Identifier {
	return dateIdentifier{
		path: path,
	}
}

type sanitizersRegex struct {
	logicalAndRegex *regexp.Regexp
	logicalOrRegex  *regexp.Regexp
	logicalEqRegex  *regexp.Regexp
	nullRegex       *regexp.Regexp
	inRegex         *regexp.Regexp
}

/*
Filter is a CEL filter expression to Spanner query parser.

It is used to parse a CEL filter expression and convert it to a Spanner statement.
*/
type Filter struct {
	identifiers     map[string]Identifier
	env             *cel.Env
	sanitizersRegex *sanitizersRegex
}

/*
NewFilter creates a new Filter instance with the given identifiers.

Identifiers are used to declare common protocol buffer types for conversion.
Common identifiers are Timestamp, Duration, Date etc.
*/
func NewFilter(identifiers ...Identifier) (*Filter, error) {

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

	return &Filter{
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
*/
func (f *Filter) DeclareIdentifier(identifier Identifier) error {
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

func (f *Filter) sanitize(filter string) string {
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

	filter.Parse("Proto.effective_date.year > 2021 AND create_time > timestamp('2021-01-01T00:00:00Z') OR expire_after > duration('1h')")
	filter.Parse("key = 'resources/1' OR Proto.effective_date = date('2021-01-01')")
	filter.Parse("Proto.state = 'ACTIVE'"
	filter.Parse("key IN ['resources/1', 'resources/2']")
	filter.Parse("effective_date != null)
	filter.Parse("count >= 10)
*/
func (f *Filter) Parse(filter string) (*spanner.Statement, error) {
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

	return &spanner.Statement{
		SQL:    sql,
		Params: params,
	}, nil
}
