package protobundle

import (
	"errors"
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Bundle is the proto schema a Spanner database needs for its PROTO
// columns: the files that declare the types, and the type names.
type Bundle struct {
	files []protoreflect.FileDescriptor // each after its imports
	types []string                      // sorted full names
}

// New collects every message and enum reachable from roots by following
// message- and enum-typed fields, plus the enums declared in each reached
// message. Map-entry messages are skipped; their key and value types are
// still followed. Duplicate roots are merged. New returns an error when
// roots is empty or holds a nil message.
func New(roots ...proto.Message) (*Bundle, error) {
	if len(roots) == 0 {
		return nil, errors.New("protobundle: no root messages")
	}
	c := &collector{
		seenTypes: map[protoreflect.FullName]bool{},
		seenFiles: map[string]bool{},
	}
	for i, r := range roots {
		if r == nil {
			return nil, fmt.Errorf("protobundle: root %d is nil", i)
		}
		c.message(r.ProtoReflect().Descriptor())
	}
	slices.Sort(c.types)
	return &Bundle{files: c.files, types: c.types}, nil
}

// Types returns the sorted full names of every type in the bundle. The
// slice is a copy.
func (b *Bundle) Types() []string { return slices.Clone(b.types) }

// collector accumulates types and files during New's walk.
type collector struct {
	seenTypes map[protoreflect.FullName]bool
	seenFiles map[string]bool
	types     []string
	files     []protoreflect.FileDescriptor
}

// message adds md (unless it is a map entry), its nested enums and its
// file, then recurses into its message- and enum-typed fields.
func (c *collector) message(md protoreflect.MessageDescriptor) {
	if !md.IsMapEntry() {
		if c.seenTypes[md.FullName()] {
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
			if !c.seenTypes[f.Enum().FullName()] {
				c.add(f.Enum())
			}
		}
	}
}

// add records one type and its file.
func (c *collector) add(d protoreflect.Descriptor) {
	c.seenTypes[d.FullName()] = true
	c.types = append(c.types, string(d.FullName()))
	c.file(d.ParentFile())
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
