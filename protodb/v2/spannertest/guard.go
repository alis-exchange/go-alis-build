package spannertest

import (
	"fmt"
	"reflect"
	"strings"

	"cloud.google.com/go/spanner/spansql"
	"go.alis.build/protodb/v2/protobundle"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// int32Kinds are the proto field kinds the emulator cannot read in DDL
// expressions or queries. A probe against emulator 1.5.57 on 2026-10-07
// showed a STORED generated column reading any of them crashes every
// write; int64 and enum fields work.
var int32Kinds = map[protoreflect.Kind]bool{
	protoreflect.Int32Kind:    true,
	protoreflect.Uint32Kind:   true,
	protoreflect.Sint32Kind:   true,
	protoreflect.Fixed32Kind:  true,
	protoreflect.Sfixed32Kind: true,
}

// pathExpType is the type of a dotted field path in a spansql expression.
var pathExpType = reflect.TypeFor[spansql.PathExp]()

// checkDDL parses each statement with spansql and reports every generated
// column or CHECK constraint, in CREATE TABLE or ALTER TABLE ADD COLUMN /
// ADD CONSTRAINT, that reads a 32-bit integer field through one of the
// table's proto columns. The error names the table, the column or
// constraint, the path and the field. Column and field names match
// case-insensitively, as in GoogleSQL, and proto types resolve against the
// bundle's own descriptors.
//
// Statements spansql cannot parse (in v1.88.0: ARRAY<proto> columns,
// schema-qualified table names, a backticked path head, parenthesised
// paths) are returned in unchecked rather than treated as safe. A nil
// bundle checks nothing.
func checkDDL(ddl []string, bundle *protobundle.Bundle) (unchecked []string, err error) {
	if bundle == nil {
		return nil, nil
	}
	// Proto columns per table, both lower-cased, kept across statements so
	// ALTER TABLE sees the columns an earlier CREATE TABLE declared.
	tables := map[string]map[string]protoreflect.MessageDescriptor{}
	var problems []string
	check := func(table, def string, expr spansql.Expr) {
		cols := tables[strings.ToLower(table)]
		for _, path := range exprPaths(expr) {
			md, ok := cols[strings.ToLower(path[0])]
			if !ok {
				continue
			}
			if f := leaf(md, path[1:]); f != nil && int32Kinds[f.Kind()] {
				problems = append(problems, fmt.Sprintf("table %s, %s: %s reads %s field %s",
					table, def, strings.Join(path, "."), f.Kind(), f.FullName()))
			}
		}
	}
	checkConstraint := func(table string, tc spansql.TableConstraint) {
		c, ok := tc.Constraint.(spansql.Check)
		if !ok {
			return
		}
		name := string(tc.Name)
		if name == "" {
			name = "CHECK"
		}
		check(table, "constraint "+name, c.Expr)
	}
	for _, stmt := range ddl {
		parsed, perr := spansql.ParseDDLStmt(stmt)
		if perr != nil {
			unchecked = append(unchecked, fmt.Sprintf("%q: %v", stmt, perr))
			continue
		}
		switch s := parsed.(type) {
		case *spansql.CreateTable:
			table := string(s.Name)
			cols := map[string]protoreflect.MessageDescriptor{}
			for _, c := range s.Columns {
				addProtoColumn(cols, bundle, c)
			}
			tables[strings.ToLower(table)] = cols
			for _, c := range s.Columns {
				check(table, "column "+string(c.Name), c.Generated)
			}
			for _, tc := range s.Constraints {
				checkConstraint(table, tc)
			}
		case *spansql.AlterTable:
			table := string(s.Name)
			switch a := s.Alteration.(type) {
			case spansql.AddColumn:
				cols := tables[strings.ToLower(table)]
				if cols == nil {
					cols = map[string]protoreflect.MessageDescriptor{}
					tables[strings.ToLower(table)] = cols
				}
				addProtoColumn(cols, bundle, a.Def)
				check(table, "column "+string(a.Def.Name), a.Def.Generated)
			case spansql.AddConstraint:
				checkConstraint(table, a.Constraint)
			}
		}
	}
	if len(problems) == 0 {
		return unchecked, nil
	}
	return unchecked, fmt.Errorf("%s; writes to these tables would fail with \"Unexpected error in RPC handling\":\n  %s",
		emulatorHint, strings.Join(problems, "\n  "))
}

// addProtoColumn records c in cols when it is a single (non-ARRAY) column
// whose type is a message in bundle.
func addProtoColumn(cols map[string]protoreflect.MessageDescriptor, bundle *protobundle.Bundle, c spansql.ColumnDef) {
	if c.Type.ProtoRef == "" || c.Type.Array {
		return
	}
	if md, ok := bundle.Lookup(c.Type.ProtoRef).(protoreflect.MessageDescriptor); ok {
		cols[strings.ToLower(string(c.Name))] = md
	}
}

// exprPaths returns every dotted field path in expr, each as its
// segments. spansql has no expression walker, so this walks the tree with
// reflect: any struct, slice or interface may hold a PathExp.
func exprPaths(expr spansql.Expr) [][]string {
	var out [][]string
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		if v.Type() == pathExpType {
			path := make([]string, v.Len())
			for i := range v.Len() {
				path[i] = v.Index(i).String()
			}
			out = append(out, path)
			return
		}
		switch v.Kind() {
		case reflect.Interface, reflect.Pointer:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			for i := range v.NumField() {
				walk(v.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := range v.Len() {
				walk(v.Index(i))
			}
		}
	}
	if expr != nil {
		walk(reflect.ValueOf(expr))
	}
	return out
}

// leaf walks names through md's fields, matching case-insensitively, and
// returns the last field, or nil when a name does not resolve or a
// non-message field has children.
func leaf(md protoreflect.MessageDescriptor, names []string) protoreflect.FieldDescriptor {
	var f protoreflect.FieldDescriptor
	for i, n := range names {
		if md == nil {
			return nil
		}
		if f = fieldByNameFold(md, n); f == nil {
			return nil
		}
		if i < len(names)-1 {
			md = f.Message()
		}
	}
	return f
}

// fieldByNameFold returns md's field named name, ignoring case, or nil.
func fieldByNameFold(md protoreflect.MessageDescriptor, name string) protoreflect.FieldDescriptor {
	if f := md.Fields().ByName(protoreflect.Name(name)); f != nil {
		return f
	}
	fields := md.Fields()
	for i := range fields.Len() {
		if strings.EqualFold(string(fields.Get(i).Name()), name) {
			return fields.Get(i)
		}
	}
	return nil
}
