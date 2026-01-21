package filtering

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/traits"
	"google.golang.org/protobuf/reflect/protoreflect"
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

// anyIdentifier represents a field that can hold any type.
// Used internally for fields where the type is not known at compile time.
type anyIdentifier struct {
	path string
}

func (t anyIdentifier) envType() *cel.Type {
	return cel.AnyType
}
func (t anyIdentifier) Path() string {
	return t.path
}

// boolIdentifier represents a boolean field.
// Used internally for fields containing true/false values.
type boolIdentifier struct {
	path string
}

func (t boolIdentifier) envType() *cel.Type {
	return cel.BoolType
}
func (t boolIdentifier) Path() string {
	return t.path
}

// bytesIdentifier represents a bytes/binary field.
// Used internally for fields containing raw byte data.
type bytesIdentifier struct {
	path string
}

func (t bytesIdentifier) envType() *cel.Type {
	return cel.BytesType
}
func (t bytesIdentifier) Path() string {
	return t.path
}

// doubleIdentifier represents a double/float64 field.
// Used internally for fields containing floating-point numbers.
type doubleIdentifier struct {
	path string
}

func (t doubleIdentifier) envType() *cel.Type {
	return cel.DoubleType
}
func (t doubleIdentifier) Path() string {
	return t.path
}

// durationIdentifier represents a google.protobuf.Duration field.
// When used in comparisons, the field is converted to seconds using:
// (field.seconds + IFNULL(field.nanos,0) / 1e9)
type durationIdentifier struct {
	path string
}

func (t durationIdentifier) envType() *cel.Type {
	return cel.DurationType
}
func (t durationIdentifier) Path() string {
	return t.path
}

// intIdentifier represents an int64 field.
// Used internally for fields containing integer values.
type intIdentifier struct {
	path string
}

func (t intIdentifier) envType() *cel.Type {
	return cel.IntType
}
func (t intIdentifier) Path() string {
	return t.path
}

// nullIdentifier represents a nullable field.
// Used internally for fields that may contain null values.
type nullIdentifier struct {
	path string
}

func (t nullIdentifier) envType() *cel.Type {
	return cel.NullType
}
func (t nullIdentifier) Path() string {
	return t.path
}

// stringIdentifier represents a string field.
// Used internally for fields containing text values.
type stringIdentifier struct {
	path string
}

func (t stringIdentifier) envType() *cel.Type {
	return cel.StringType
}
func (t stringIdentifier) Path() string {
	return t.path
}

// timestampIdentifier represents a google.protobuf.Timestamp field.
// When used in comparisons, the field is converted to a Spanner TIMESTAMP using:
// TIMESTAMP_ADD(TIMESTAMP_SECONDS(field.seconds), INTERVAL ... MICROSECOND)
type timestampIdentifier struct {
	path string
}

func (t timestampIdentifier) envType() *cel.Type {
	return cel.TimestampType
}
func (t timestampIdentifier) Path() string {
	return t.path
}

// dateIdentifier represents a google.type.Date field.
// When used in comparisons, the field is converted to a Spanner DATE using:
// DATE(field.year, field.month, field.day)
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

// uintIdentifier represents an unsigned int64 field.
// Used internally for fields containing unsigned integer values.
type uintIdentifier struct {
	path string
}

func (t uintIdentifier) envType() *cel.Type {
	return cel.UintType
}
func (t uintIdentifier) Path() string {
	return t.path
}

// listIdentifier represents a repeated/array field.
// Used internally for fields containing lists of values.
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

// mapIdentifier represents a map field.
// Used internally for fields containing key-value pairs.
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

// reservedIdentifier represents a column with a SQL reserved keyword name.
// When used in SQL, the column name is wrapped in backticks to avoid syntax errors.
// Example: `select`, `from`, `group`
type reservedIdentifier struct {
	name string
}

func (t reservedIdentifier) envType() *cel.Type {
	return types.NewOpaqueType(t.name)
}
func (t reservedIdentifier) Path() string {
	return t.name
}

// ProtoEnum defines the interface for protocol buffer enum types.
// This interface is used internally for type checking enum values.
// It matches the methods generated by protoc for enum types.
type ProtoEnum interface {
	Enum() *ProtoEnum
	String() string
	Descriptor() protoreflect.EnumDescriptor
	Type() protoreflect.EnumType
	Number() protoreflect.EnumNumber
	EnumDescriptor() ([]byte, []int)
}

// enumStringIdentifier represents a protocol buffer enum field to be compared as a string.
// When used in comparisons, the field is cast to STRING: CAST(field AS STRING)
// This allows filtering by enum name (e.g., "ACTIVE", "PENDING").
type enumStringIdentifier struct {
	path string // Field path (e.g., "Proto.status")
	name string // Fully qualified enum type name
}

func (t enumStringIdentifier) envType() *cel.Type {
	return cel.ObjectType(t.name, traits.AdderType|
		traits.ComparerType|
		traits.NegatorType|
		traits.ReceiverType|
		traits.SubtractorType)
}
func (t enumStringIdentifier) Path() string {
	return t.path
}

// enumIntegerIdentifier represents a protocol buffer enum field to be compared as an integer.
// When used in comparisons, the field is cast to INT64: CAST(field AS INT64)
// This allows filtering by enum number (e.g., 0, 1, 2).
type enumIntegerIdentifier struct {
	path string // Field path (e.g., "Proto.priority")
	name string // Fully qualified enum type name
}

func (t enumIntegerIdentifier) envType() *cel.Type {
	return cel.ObjectType(t.name, traits.AdderType|
		traits.ComparerType|
		traits.NegatorType|
		traits.ReceiverType|
		traits.SubtractorType)
}
func (t enumIntegerIdentifier) Path() string {
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

/*
Reserved allows for the querying of columns with reserved keywords.
It instructs the parser to wrap the column names with backticks(`).
This is only required if you have a column with a reserved keyword

It takes in the name of the column.
*/
func Reserved(name string) Identifier {
	return reservedIdentifier{
		name: name,
	}
}

/*
EnumString allows for the querying of enum fields in protocol buffers.
It enables casting of enum fields to their string representation in CEL.

It takes in the path to the enum field and the enum type.

Example:

	var status pb.Proto_Status
	EnumString("Proto.status", string(status.Descriptor().FullName()))
*/
func EnumString(path string, name string) Identifier {
	return enumStringIdentifier{
		path: path,
		name: name,
	}
}

/*
EnumInteger allows for the querying of enum fields in protocol buffers.
It enables casting of enum fields to their integer representation in CEL.

It takes in the path to the enum field and the enum type.

Example:

	var status pb.Proto_Status
	EnumInteger("Proto.status", string(status.Descriptor().FullName()))
*/
func EnumInteger(path string, name string) Identifier {
	return enumIntegerIdentifier{
		path: path,
		name: name,
	}
}
