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

type reservedIdentifier struct {
	name string
}

func (t reservedIdentifier) envType() *cel.Type {
	return types.NewOpaqueType(t.name)
}
func (t reservedIdentifier) Path() string {
	return t.name
}

type ProtoEnum interface {
	Enum() *ProtoEnum
	String() string
	Descriptor() protoreflect.EnumDescriptor
	Type() protoreflect.EnumType
	Number() protoreflect.EnumNumber
	EnumDescriptor() ([]byte, []int)
}

type enumStringIdentifier struct {
	path string
	name string
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

type enumIntegerIdentifier struct {
	path string
	name string
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
