package protobundle

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Bundle is the proto schema a Spanner database needs for its PROTO and
// ENUM columns: the files that declare the types, and the type names.
type Bundle struct {
	files       []protoreflect.FileDescriptor // each after its imports
	types       []string                      // sorted full names
	descriptors map[protoreflect.FullName]protoreflect.Descriptor
}

// New collects every message and enum reachable from roots by following
// message- and enum-typed fields, plus the enums declared in each reached
// message and every message containing a reached nested type. Map-entry
// messages are skipped; their key and value types are still followed.
// Duplicate roots are merged. New returns an error when roots is empty or
// holds a nil interface; a typed nil pointer such as (*iampb.Policy)(nil)
// is accepted, since only its descriptor is read.
func New(roots ...proto.Message) (*Bundle, error) {
	if len(roots) == 0 {
		return nil, errors.New("protobundle: no root messages")
	}
	descriptors := make([]protoreflect.Descriptor, len(roots))
	for i, r := range roots {
		if r == nil {
			return nil, fmt.Errorf("protobundle: root %d is nil", i)
		}
		descriptors[i] = r.ProtoReflect().Descriptor()
	}
	return NewFromDescriptors(descriptors...)
}

// NewFromDescriptors is New for roots given as descriptors. A root may be a
// message or an enum, so a top-level enum used only by an ENUM column can
// be bundled, e.g. NewFromDescriptors(librarypb.Genre(0).Descriptor()). It
// returns an error when roots is empty, or holds nil or a descriptor of any
// other kind.
func NewFromDescriptors(roots ...protoreflect.Descriptor) (*Bundle, error) {
	if len(roots) == 0 {
		return nil, errors.New("protobundle: no root descriptors")
	}
	c := &collector{
		seenTypes: map[protoreflect.FullName]protoreflect.Descriptor{},
		seenFiles: map[string]bool{},
	}
	for i, r := range roots {
		switch d := r.(type) {
		case protoreflect.MessageDescriptor:
			c.message(d)
		case protoreflect.EnumDescriptor:
			c.add(d)
		case nil:
			return nil, fmt.Errorf("protobundle: root %d is nil", i)
		default:
			return nil, fmt.Errorf("protobundle: root %d (%s) is neither a message nor an enum", i, d.FullName())
		}
	}
	slices.Sort(c.types)
	return &Bundle{files: c.files, types: c.types, descriptors: c.seenTypes}, nil
}

// Lookup returns the descriptor of the bundled message or enum named
// fullName, or nil when the bundle does not list it. It answers from the
// bundle's own descriptors, which need not be in the global registry.
func (b *Bundle) Lookup(fullName string) protoreflect.Descriptor {
	return b.descriptors[protoreflect.FullName(fullName)]
}

// Types returns the sorted full names of every type in the bundle. The
// slice is a copy.
func (b *Bundle) Types() []string { return slices.Clone(b.types) }

// Descriptors returns the marshalled FileDescriptorSet for the bundle's
// files, each listed after the files it imports, ready for the
// ProtoDescriptors field of CreateDatabaseRequest or
// UpdateDatabaseDdlRequest.
func (b *Bundle) Descriptors() ([]byte, error) {
	set := &descriptorpb.FileDescriptorSet{
		File: make([]*descriptorpb.FileDescriptorProto, 0, len(b.files)),
	}
	for _, fd := range b.files {
		set.File = append(set.File, protodesc.ToFileDescriptorProto(fd))
	}
	return proto.Marshal(set)
}

// CreateStatement returns the CREATE PROTO BUNDLE statement naming every
// type in the bundle, in Types order.
func (b *Bundle) CreateStatement() string {
	quoted := make([]string, len(b.types))
	for i, t := range b.types {
		quoted[i] = "`" + t + "`"
	}
	return "CREATE PROTO BUNDLE (" + strings.Join(quoted, ", ") + ")"
}

// collector accumulates types and files during New's walk.
type collector struct {
	seenTypes map[protoreflect.FullName]protoreflect.Descriptor
	seenFiles map[string]bool
	types     []string
	files     []protoreflect.FileDescriptor
}

// message adds md (unless it is a map entry), its nested enums and its
// file, then recurses into its message- and enum-typed fields.
func (c *collector) message(md protoreflect.MessageDescriptor) {
	if !md.IsMapEntry() {
		if _, ok := c.seenTypes[md.FullName()]; ok {
			return
		}
		c.add(md)
		for i := range md.Enums().Len() {
			c.add(md.Enums().Get(i))
		}
	}
	for i := range md.Fields().Len() {
		f := md.Fields().Get(i)
		switch {
		case f.Message() != nil:
			c.message(f.Message())
		case f.Enum() != nil:
			c.add(f.Enum())
		}
	}
}

// add records one type, its file and every message containing it, once. A
// nested enum can be reached by a field before its parent message is
// visited, so every path goes through this check.
func (c *collector) add(d protoreflect.Descriptor) {
	if _, ok := c.seenTypes[d.FullName()]; ok {
		return
	}
	c.seenTypes[d.FullName()] = d
	c.types = append(c.types, string(d.FullName()))
	c.file(d.ParentFile())
	// Spanner rejects a bundle that names a nested type without the
	// message containing it, so walk up to the top-level message.
	if parent, ok := d.Parent().(protoreflect.MessageDescriptor); ok {
		c.message(parent)
	}
}

// file appends fd after all of its imports, once.
func (c *collector) file(fd protoreflect.FileDescriptor) {
	if c.seenFiles[fd.Path()] {
		return
	}
	c.seenFiles[fd.Path()] = true
	for i := range fd.Imports().Len() {
		c.file(fd.Imports().Get(i).FileDescriptor)
	}
	c.files = append(c.files, fd)
}
