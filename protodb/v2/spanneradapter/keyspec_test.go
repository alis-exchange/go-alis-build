package spanneradapter

import (
	"reflect"
	"testing"

	"go.alis.build/protodb/v2"
)

// sessionKey is the brief's canonical tagged-struct example: a multi-column
// key with three pdb-tagged string fields.
type sessionKey struct {
	SessionID string `pdb:"session_id"`
	AppName   string `pdb:"app_name"`
	UserID    string `pdb:"user_id"`
}

func (k sessionKey) KeyValues() []any { return KeyValuesOf(k) }

// twoStr is used to test that Encode's canonical string is injective across
// field boundaries (e.g. "a:b","c" must not collide with "a","b:c").
type twoStr struct {
	A string `pdb:"a"`
	B string `pdb:"b"`
}

func (k twoStr) KeyValues() []any { return KeyValuesOf(k) }

// badKey has an untagged exported field, which KeySpecFor must reject.
type badKey struct {
	A string
}

func (k badKey) KeyValues() []any { return KeyValuesOf(k) }

// badKeyWrap wraps badKey so it satisfies protodb.Key while keeping the
// untagged struct itself distinct (mirrors the brief's badKeyWrap helper).
type badKeyWrap = badKey

// unsupportedKey has a field type KeySpecFor does not support.
type unsupportedKey struct {
	A []byte `pdb:"a"`
}

func (k unsupportedKey) KeyValues() []any { return KeyValuesOf(k) }

func TestKeyValuesOf(t *testing.T) {
	k := sessionKey{"s", "a", "u"}
	if !reflect.DeepEqual(k.KeyValues(), []any{"s", "a", "u"}) {
		t.Fatal(k.KeyValues())
	}
}

func TestKeySpecForColumns(t *testing.T) {
	spec, err := KeySpecFor[sessionKey](WithParentColumns(1))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec.Columns(), []string{"session_id", "app_name", "user_id"}) {
		t.Fatal(spec.Columns())
	}
	sql, params := spec.ParentFilter("s1")
	if sql != "session_id = @parent" || params["parent"] != "s1" {
		t.Fatalf("%q %v", sql, params)
	}
	if sql, _ := spec.ParentFilter(""); sql != "" {
		t.Fatal("empty parent = no filter")
	}
}

func TestKeySpecForRejectsUntagged(t *testing.T) {
	if _, err := KeySpecFor[badKeyWrap](); err == nil {
		t.Fatal("untagged exported field must error")
	}
}

func TestKeySpecForRejectsUnsupportedFieldType(t *testing.T) {
	if _, err := KeySpecFor[unsupportedKey](); err == nil {
		t.Fatal("unsupported field type must error")
	}
}

func TestKeySpecForRejectsMultiColumnParent(t *testing.T) {
	if _, err := KeySpecFor[sessionKey](WithParentColumns(2)); err == nil {
		t.Fatal("multi-column parent must error at construction")
	}
}

func TestStringKeySpec(t *testing.T) {
	spec := StringKeySpec("key")
	if !reflect.DeepEqual(spec.Columns(), []string{"key"}) {
		t.Fatal()
	}
	sql, params := spec.ParentFilter("resources/")
	if sql != "STARTS_WITH(`key`, @parent)" || params["parent"] != "resources/" {
		t.Fatalf("%q", sql)
	}
	if sql, _ := spec.ParentFilter(""); sql != "" {
		t.Fatal("empty parent = no filter")
	}
}

func TestStringKeyImplementsProtodbKey(t *testing.T) {
	var _ protodb.Key = StringKey("resources/1")
	k := StringKey("resources/1")
	if !reflect.DeepEqual(k.KeyValues(), []any{"resources/1"}) {
		t.Fatal(k.KeyValues())
	}
}

func TestEncodeInjective(t *testing.T) {
	spec, err := KeySpecFor[twoStr]()
	if err != nil {
		t.Fatal(err)
	}
	a, err := spec.Encode(twoStr{"a:b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := spec.Encode(twoStr{"a", "b:c"})
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("canonical encoding must be injective")
	}
}

func TestEncodeEqualForEqualKeys(t *testing.T) {
	spec, err := KeySpecFor[twoStr]()
	if err != nil {
		t.Fatal(err)
	}
	a, err := spec.Encode(twoStr{"x", "y"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := spec.Encode(twoStr{"x", "y"})
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("equal keys must encode equally: %q != %q", a, b)
	}
}

func TestKeySpecForDerefsPointerKind(t *testing.T) {
	// K may be a pointer to a struct; KeySpecFor derefs it at construction.
	spec, err := KeySpecFor[*sessionKey]()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec.Columns(), []string{"session_id", "app_name", "user_id"}) {
		t.Fatal(spec.Columns())
	}
}
