package spannertest

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"go.alis.build/protodb/v2/protobundle"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
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

var (
	// createTableRE captures a CREATE TABLE's name and its column list.
	createTableRE = regexp.MustCompile("(?is)^\\s*CREATE\\s+TABLE\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?" +
		"(`[^`]+`|\\w+)\\s*\\((.*)\\)\\s*PRIMARY\\s+KEY")
	// pathRE matches a dotted field path whose head may be backticked.
	pathRE = regexp.MustCompile("(`[^`]+`|[A-Za-z_][A-Za-z0-9_]*)((?:\\.[A-Za-z_][A-Za-z0-9_]*)+)")
	// exprRE finds the opening of a generated-column or CHECK expression.
	exprRE = regexp.MustCompile(`(?i)\b(AS|CHECK)\s*\(`)
)

// checkDDL reports every generated-column or CHECK expression in a CREATE
// TABLE statement that reads a 32-bit integer field through one of the
// table's proto columns, naming the table, the generated column or
// constraint, the path and the field. A proto column is
// one whose type is a backticked type from bundle. Paths it cannot resolve
// are ignored, and a nil bundle checks nothing.
func checkDDL(ddl []string, bundle *protobundle.Bundle) error {
	if bundle == nil {
		return nil
	}
	messages := map[string]protoreflect.MessageDescriptor{}
	for _, name := range bundle.Types() {
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
		if md, ok := d.(protoreflect.MessageDescriptor); err == nil && ok {
			messages[name] = md
		}
	}
	var problems []string
	for _, stmt := range ddl {
		m := createTableRE.FindStringSubmatch(stmt)
		if m == nil {
			continue
		}
		table := strings.Trim(m[1], "`")
		defs := splitTopLevel(m[2])
		cols := protoColumns(defs, messages)
		for _, def := range defs {
			for _, expr := range expressions(def) {
				for _, p := range pathRE.FindAllStringSubmatch(blankLiterals(expr), -1) {
					md, ok := cols[strings.Trim(p[1], "`")]
					if !ok {
						continue
					}
					if f := leaf(md, strings.Split(p[2][1:], ".")); f != nil && int32Kinds[f.Kind()] {
						problems = append(problems, fmt.Sprintf("table %s, column %s: %s reads %s field %s",
							table, definitionName(def), p[0], f.Kind(), f.FullName()))
					}
				}
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s; writes to these tables would fail with \"Unexpected error in RPC handling\":\n  %s",
		emulatorHint, strings.Join(problems, "\n  "))
}

// protoColumns maps each column whose type is a backticked bundle message
// to that message's descriptor.
func protoColumns(defs []string, messages map[string]protoreflect.MessageDescriptor) map[string]protoreflect.MessageDescriptor {
	cols := map[string]protoreflect.MessageDescriptor{}
	for _, def := range defs {
		fields := strings.Fields(def)
		if len(fields) < 2 || !strings.HasPrefix(fields[1], "`") {
			continue
		}
		if md, ok := messages[strings.Trim(fields[1], "`,")]; ok {
			cols[strings.Trim(fields[0], "`")] = md
		}
	}
	return cols
}

// definitionName returns the name a column or constraint definition
// declares: the column name, the name after CONSTRAINT, or CHECK for an
// unnamed check constraint.
func definitionName(def string) string {
	fields := strings.Fields(def)
	switch {
	case len(fields) == 0:
		return ""
	case strings.EqualFold(fields[0], "CONSTRAINT") && len(fields) > 1:
		return strings.Trim(fields[1], "`")
	case strings.HasPrefix(strings.ToUpper(fields[0]), "CHECK"):
		return "CHECK"
	}
	return strings.Trim(fields[0], "`")
}

// leaf walks names through md's fields and returns the last field, or nil
// when a name does not resolve or a non-message field has children.
func leaf(md protoreflect.MessageDescriptor, names []string) protoreflect.FieldDescriptor {
	var f protoreflect.FieldDescriptor
	for i, n := range names {
		if md == nil {
			return nil
		}
		f = md.Fields().ByName(protoreflect.Name(n))
		if f == nil {
			return nil
		}
		if i < len(names)-1 {
			md = f.Message()
		}
	}
	return f
}

// expressions returns the parenthesised bodies after each AS ( or CHECK (
// in one column or constraint definition.
func expressions(def string) []string {
	var out []string
	for _, loc := range exprRE.FindAllStringIndex(def, -1) {
		open := loc[1] - 1
		if end := matchParen(def, open); end > open {
			out = append(out, def[open+1:end])
		}
	}
	return out
}

// matchParen returns the index of the parenthesis closing s[open], skipping
// quoted and backticked text, or -1 when it is never closed.
func matchParen(s string, open int) int {
	depth := 0
	var quote byte
	for i := open; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// blankLiterals replaces the contents of single- and double-quoted string
// literals with spaces, so dotted text inside a literal is never read as a
// field path. Backticked identifiers are kept.
func blankLiterals(s string) string {
	b := []byte(s)
	var quote byte
	for i, c := range b {
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				b[i] = ' '
			}
		case c == '\'' || c == '"':
			quote = c
		}
	}
	return string(b)
}

// splitTopLevel splits a CREATE TABLE column list at commas outside
// parentheses, quotes and backticks, dropping empty entries.
func splitTopLevel(s string) []string {
	var out []string
	depth, start := 0, 0
	var quote byte
	for i := range len(s) {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ',' && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return slices.DeleteFunc(out, func(d string) bool { return strings.TrimSpace(d) == "" })
}
