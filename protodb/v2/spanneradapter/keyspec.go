package spanneradapter

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"cloud.google.com/go/spanner"
	"go.alis.build/protodb/v2"
)

// KeySpec declares the structure of a table's key: which columns compose
// it, how to scope a List/Stream query to a parent resource, how to decode
// a key out of a Spanner row, and how to encode a key to a canonical string
// for in-process comparison. The library ships two implementations —
// StringKeySpec for the common single string-column case and KeySpecFor
// for tagged structs — and consumers may implement KeySpec directly for
// exotic key shapes.
type KeySpec interface {
	// Columns returns the key's columns, in key order.
	Columns() []string
	// ParentFilter returns a SQL boolean expression (and its bound
	// params) that scopes a query to rows whose key falls under parent.
	// An empty parent means "no parent scoping": ("", nil) is returned.
	ParentFilter(parent string) (sql string, params map[string]any)
	// Decode builds a protodb.Key by reading this spec's columns out of
	// row.
	Decode(row *spanner.Row) (protodb.Key, error)
	// Encode returns a canonical string representation of key, injective
	// over the values KeySpec supports. The result is meaningful only
	// for equality comparison within one process (e.g. joining batch
	// read results back to requested keys) — it must never be decoded
	// back into a key.
	Encode(key protodb.Key) (string, error)
}

// encodeKey implements the KeySpec.Encode contract shared by every
// KeySpec in this package: json.Marshal of the key's ordered values. A
// JSON array of typed scalars is injective for the field types KeySpecFor
// supports (string, int64, bool, float64, time.Time), because JSON encodes
// each value with its type-specific syntax (quoted strings, bare numbers,
// bare booleans, RFC3339 timestamps) rather than concatenating them, so
// values can never bleed across a field boundary. The output is compared
// only for equality in-process; it is never decoded back into a key.
func encodeKey(key protodb.Key) (string, error) {
	b, err := json.Marshal(key.KeyValues())
	if err != nil {
		return "", fmt.Errorf("spanneradapter: encode key: %w", err)
	}
	return string(b), nil
}

// StringKey is a single string-column key. It implements protodb.Key
// directly, so consumers with a plain string identity (e.g. a resource
// name) don't need a wrapper type.
type StringKey string

// KeyValues implements protodb.Key.
func (k StringKey) KeyValues() []any { return []any{string(k)} }

// stringKeySpec is the KeySpec for a single string column, with parent
// scoping expressed as a prefix match (STARTS_WITH) rather than equality —
// the natural scoping for hierarchical resource-name-shaped keys.
type stringKeySpec struct {
	column string
}

// StringKeySpec returns a KeySpec for a table keyed by a single string
// column. ParentFilter scopes to rows whose column starts with parent
// (STARTS_WITH), which fits resource-name-shaped keys such as
// "parent/123/children/456".
func StringKeySpec(column string) KeySpec {
	return &stringKeySpec{column: column}
}

func (s *stringKeySpec) Columns() []string { return []string{s.column} }

func (s *stringKeySpec) ParentFilter(parent string) (string, map[string]any) {
	if parent == "" {
		return "", nil
	}
	return fmt.Sprintf("STARTS_WITH(`%s`, @parent)", s.column), map[string]any{"parent": parent}
}

func (s *stringKeySpec) Decode(row *spanner.Row) (protodb.Key, error) {
	var v string
	if err := row.ColumnByName(s.column, &v); err != nil {
		return nil, fmt.Errorf("spanneradapter: decode string key column %q: %w", s.column, err)
	}
	return StringKey(v), nil
}

func (s *stringKeySpec) Encode(key protodb.Key) (string, error) { return encodeKey(key) }

// KeySpecOption configures KeySpecFor.
type KeySpecOption func(*keySpecOpts)

type keySpecOpts struct {
	parentCols int
}

// WithParentColumns sets how many leading key columns ParentFilter matches
// against the parent argument. Only n==1 is supported in this version:
// ParentFilter equates the first column against @parent. n>1 is rejected
// at construction — multi-column parent scoping is not implemented yet.
// If WithParentColumns is not passed, KeySpecFor defaults to n==1.
func WithParentColumns(n int) KeySpecOption {
	return func(o *keySpecOpts) { o.parentCols = n }
}

// fieldKind identifies the supported Spanner-scalar Go types a KeySpecFor
// field may have.
type fieldKind int

const (
	kindString fieldKind = iota
	kindInt64
	kindBool
	kindFloat64
	kindTime
)

var (
	stringType  = reflect.TypeOf("")
	int64Type   = reflect.TypeOf(int64(0))
	boolType    = reflect.TypeOf(false)
	float64Type = reflect.TypeOf(float64(0))
	timeType    = reflect.TypeOf(time.Time{})
)

func fieldKindOf(t reflect.Type) (fieldKind, error) {
	switch t {
	case stringType:
		return kindString, nil
	case int64Type:
		return kindInt64, nil
	case boolType:
		return kindBool, nil
	case float64Type:
		return kindFloat64, nil
	case timeType:
		return kindTime, nil
	default:
		return 0, fmt.Errorf("unsupported key field type %s (supported: string, int64, bool, float64, time.Time)", t)
	}
}

// specField is one column of a tagged-struct KeySpec, resolved once at
// construction.
type specField struct {
	fieldIndex int
	column     string
	kind       fieldKind
}

// taggedKeySpec is the KeySpec built by KeySpecFor. It holds the reflected
// field table computed once at construction; Decode and Encode never
// reflect over the struct's tags again.
type taggedKeySpec struct {
	structType reflect.Type // always the struct type, even when K is a pointer
	ptrKind    bool         // true if K is a pointer to structType
	fields     []specField
}

// KeySpecFor builds a KeySpec for a struct key type K by reflecting once
// over its exported, `pdb`-tagged fields: the tag value is the column name
// and field order is column order. K may be a struct or a pointer to one.
// Every exported field of K must carry a non-empty `pdb` tag, and its type
// must be one of string, int64, bool, float64, or time.Time — otherwise
// KeySpecFor returns an error. Decode builds a K via reflection and typed
// per-column reads (row.ColumnByName); Encode is the shared
// json.Marshal(key.KeyValues()) canonical string.
func KeySpecFor[K protodb.Key](opts ...KeySpecOption) (KeySpec, error) {
	o := keySpecOpts{parentCols: 1}
	for _, opt := range opts {
		opt(&o)
	}
	if o.parentCols > 1 {
		return nil, fmt.Errorf("spanneradapter: KeySpecFor: WithParentColumns(%d): multi-column parents are not supported in this version", o.parentCols)
	}

	typ := reflect.TypeFor[K]()
	ptrKind := typ.Kind() == reflect.Pointer
	structType := typ
	if ptrKind {
		structType = typ.Elem()
	}
	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("spanneradapter: KeySpecFor: %s is not a struct or a pointer to a struct", typ)
	}

	var fields []specField
	for i := 0; i < structType.NumField(); i++ {
		f := structType.Field(i)
		if !f.IsExported() {
			continue
		}
		column := f.Tag.Get("pdb")
		if column == "" {
			return nil, fmt.Errorf("spanneradapter: KeySpecFor: %s.%s has no `pdb` tag", structType.Name(), f.Name)
		}
		kind, err := fieldKindOf(f.Type)
		if err != nil {
			return nil, fmt.Errorf("spanneradapter: KeySpecFor: %s.%s: %w", structType.Name(), f.Name, err)
		}
		fields = append(fields, specField{fieldIndex: i, column: column, kind: kind})
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("spanneradapter: KeySpecFor: %s has no `pdb`-tagged exported fields", structType)
	}

	return &taggedKeySpec{structType: structType, ptrKind: ptrKind, fields: fields}, nil
}

func (s *taggedKeySpec) Columns() []string {
	cols := make([]string, len(s.fields))
	for i, f := range s.fields {
		cols[i] = f.column
	}
	return cols
}

func (s *taggedKeySpec) ParentFilter(parent string) (string, map[string]any) {
	if parent == "" {
		return "", nil
	}
	return fmt.Sprintf("%s = @parent", s.fields[0].column), map[string]any{"parent": parent}
}

func (s *taggedKeySpec) Decode(row *spanner.Row) (protodb.Key, error) {
	instPtr := reflect.New(s.structType)
	for _, f := range s.fields {
		fv := instPtr.Elem().Field(f.fieldIndex)
		switch f.kind {
		case kindString:
			var v string
			if err := row.ColumnByName(f.column, &v); err != nil {
				return nil, fmt.Errorf("spanneradapter: decode key column %q: %w", f.column, err)
			}
			fv.SetString(v)
		case kindInt64:
			var v int64
			if err := row.ColumnByName(f.column, &v); err != nil {
				return nil, fmt.Errorf("spanneradapter: decode key column %q: %w", f.column, err)
			}
			fv.SetInt(v)
		case kindBool:
			var v bool
			if err := row.ColumnByName(f.column, &v); err != nil {
				return nil, fmt.Errorf("spanneradapter: decode key column %q: %w", f.column, err)
			}
			fv.SetBool(v)
		case kindFloat64:
			var v float64
			if err := row.ColumnByName(f.column, &v); err != nil {
				return nil, fmt.Errorf("spanneradapter: decode key column %q: %w", f.column, err)
			}
			fv.SetFloat(v)
		case kindTime:
			var v time.Time
			if err := row.ColumnByName(f.column, &v); err != nil {
				return nil, fmt.Errorf("spanneradapter: decode key column %q: %w", f.column, err)
			}
			fv.Set(reflect.ValueOf(v))
		}
	}
	if s.ptrKind {
		return instPtr.Interface().(protodb.Key), nil
	}
	return instPtr.Elem().Interface().(protodb.Key), nil
}

func (s *taggedKeySpec) Encode(key protodb.Key) (string, error) { return encodeKey(key) }

// KeyValuesOf returns the `pdb`-tagged field values of k, in field order.
// It's a reflection helper for consumers implementing protodb.Key on a
// tagged struct:
//
//	func (k K) KeyValues() []any { return spanneradapter.KeyValuesOf(k) }
func KeyValuesOf(k any) []any {
	v := reflect.ValueOf(k)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	var vals []any
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Tag.Get("pdb") == "" {
			continue
		}
		vals = append(vals, v.Field(i).Interface())
	}
	return vals
}
