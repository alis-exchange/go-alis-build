package memadapter

import (
	"slices"
	"strings"

	"go.alis.build/protodb/v2/filtering"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// compileFilter compiles a List/Stream filter, or returns nil for an empty
// one. See Config.ResourceColumn for when filtering is available.
func (t *Table[R]) compileFilter(filter string, params map[string]any) (*filtering.Predicate, error) {
	if strings.TrimSpace(filter) == "" {
		return nil, nil
	}
	if t.cfg.ResourceColumn == "" {
		return nil, errFilterUnimplemented
	}
	if t.parserErr != nil {
		return nil, status.Errorf(codes.InvalidArgument, "memadapter: Config.FilterIdentifiers: %v", t.parserErr)
	}
	return t.parser.Compile(filter, params)
}

// resolver returns the filter path resolver for one stored row, following
// the order documented on Config.ResourceColumn.
func (t *Table[R]) resolver(e *entry[R]) func(string) (any, error) {
	return func(path string) (any, error) {
		values := e.key.KeyValues()
		if i := slices.Index(t.cfg.KeyColumns, path); i >= 0 && i < len(values) {
			return values[i], nil
		}
		if path == "key" && len(values) > 0 {
			return values[0], nil
		}
		if path == t.cfg.ResourceColumn || strings.HasPrefix(path, t.cfg.ResourceColumn+".") {
			m, ok := any(e.resource).(proto.Message)
			if !ok {
				return nil, status.Errorf(codes.Unimplemented,
					"memadapter: filtering on %q needs a proto.Message resource, not %T", path, e.resource)
			}
			if !m.ProtoReflect().IsValid() {
				return nil, nil // a nil resource is a NULL column, and so is every path below it
			}
			if path == t.cfg.ResourceColumn {
				return m, nil
			}
			return fieldValue(m.ProtoReflect(), strings.TrimPrefix(path, t.cfg.ResourceColumn+"."), path)
		}
		if fn, ok := t.cfg.Columns[path]; ok {
			return fn(e.key, e.resource), nil
		}
		return nil, status.Errorf(codes.InvalidArgument, "memadapter: unknown filter path %q", path)
	}
}

// fieldValue walks a dotted field path through msg. It returns nil for an
// unset message field, an unset field with explicit presence, or a path
// through an unset message; a proto.Message for a message field; a
// protoreflect.EnumValueDescriptor for an enum; and the Go value for a
// scalar.
func fieldValue(msg protoreflect.Message, fieldPath, fullPath string) (any, error) {
	segments := strings.Split(fieldPath, ".")
	for i, seg := range segments {
		fd := msg.Descriptor().Fields().ByName(protoreflect.Name(seg))
		if fd == nil {
			return nil, status.Errorf(
				codes.InvalidArgument,
				"memadapter: unknown filter path %q: %s has no field %q",
				fullPath,
				msg.Descriptor().FullName(),
				seg,
			)
		}
		if fd.IsList() || fd.IsMap() {
			return nil, status.Errorf(codes.Unimplemented, "memadapter: filtering on repeated or map field %q is not supported", fullPath)
		}
		if fd.HasPresence() && !msg.Has(fd) {
			return nil, nil
		}
		v := msg.Get(fd)
		last := i == len(segments)-1
		switch {
		case fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind:
			if last {
				return v.Message().Interface(), nil
			}
			msg = v.Message()
		case !last:
			return nil, status.Errorf(codes.InvalidArgument, "memadapter: unknown filter path %q: %q is not a message", fullPath, seg)
		case fd.Kind() == protoreflect.EnumKind:
			if ev := fd.Enum().Values().ByNumber(v.Enum()); ev != nil {
				return ev, nil
			}
			return int64(v.Enum()), nil
		case fd.Kind() == protoreflect.Uint64Kind || fd.Kind() == protoreflect.Fixed64Kind:
			// Spanner reads 64-bit unsigned proto fields as INT64.
			return int64(v.Uint()), nil //nolint:gosec // same wrap-around as Spanner
		default:
			return v.Interface(), nil
		}
	}
	return nil, nil
}
