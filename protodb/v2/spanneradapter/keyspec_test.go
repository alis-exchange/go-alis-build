package spanneradapter

import (
	"reflect"
	"testing"

	"cloud.google.com/go/spanner"
	"go.alis.build/protodb/v2"
)

// sessionKey is the canonical tagged-struct key in these tests: a multi-column
// key with three pdb-tagged string fields.
type sessionKey struct {
	SessionID string `pdb:"session_id"`
	AppName   string `pdb:"app_name"`
	UserID    string `pdb:"user_id"`
}

// KeyValues implements protodb.Key.
func (k sessionKey) KeyValues() []any { return KeyValuesOf(k) }

// twoStr is used to test that Encode's canonical string is injective across
// field boundaries (e.g. "a:b","c" must not collide with "a","b:c").
type twoStr struct {
	A string `pdb:"a"`
	B string `pdb:"b"`
}

// KeyValues implements protodb.Key.
func (k twoStr) KeyValues() []any { return KeyValuesOf(k) }

// badKey has an untagged exported field, which KeySpecFor must reject.
type badKey struct {
	A string
}

// KeyValues implements protodb.Key.
func (k badKey) KeyValues() []any { return KeyValuesOf(k) }

// badKeyWrap names badKey, the untagged struct, where a test wants to read as
// "the wrapper that satisfies protodb.Key".
type badKeyWrap = badKey

// unsupportedKey has a field type KeySpecFor does not support.
type unsupportedKey struct {
	A []byte `pdb:"a"`
}

// KeyValues implements protodb.Key.
func (k unsupportedKey) KeyValues() []any { return KeyValuesOf(k) }

// unexportedTaggedKey has an unexported field that carries a stray `pdb`
// tag. KeyValuesOf must skip it (unexported fields are never valid key
// columns) rather than panic trying to read it via reflection.
type unexportedTaggedKey struct {
	Public  string `pdb:"public"`
	private string `pdb:"private"`
}

// KeyValues implements protodb.Key.
func (k unexportedTaggedKey) KeyValues() []any { return KeyValuesOf(k) }

// TestKeyValuesOf checks that KeyValuesOf returns a tagged struct's field values in field order.
func TestKeyValuesOf(t *testing.T) {
	k := sessionKey{"s", "a", "u"}
	if !reflect.DeepEqual(k.KeyValues(), []any{"s", "a", "u"}) {
		t.Fatal(k.KeyValues())
	}
}

// TestKeySpecForColumns checks that KeySpecFor takes column names from the pdb tags in field order,
// and that a one-column parent filters by equality on the first column, or not at all when empty.
func TestKeySpecForColumns(t *testing.T) {
	spec, err := KeySpecFor[sessionKey](WithParentColumns(1))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec.Columns(), []string{"session_id", "app_name", "user_id"}) {
		t.Fatal(spec.Columns())
	}
	sql, params := spec.ParentFilter("s1")
	if sql != "`session_id` = @parent" || params["parent"] != "s1" {
		t.Fatalf("%q %v", sql, params)
	}
	if sql, _ := spec.ParentFilter(""); sql != "" {
		t.Fatal("empty parent = no filter")
	}
}

// keywordParentKey has a reserved-word-named leading column, so
// ParentFilter must backtick-quote it or the emitted SQL is invalid.
type keywordParentKey struct {
	Key   string `pdb:"key"`
	Order string `pdb:"order"`
}

// KeyValues implements protodb.Key.
func (k keywordParentKey) KeyValues() []any { return KeyValuesOf(k) }

// TestKeySpecForParentFilterQuotesKeywordColumn checks that ParentFilter backtick-quotes a
// leading column named after a reserved word.
func TestKeySpecForParentFilterQuotesKeywordColumn(t *testing.T) {
	spec, err := KeySpecFor[keywordParentKey]()
	if err != nil {
		t.Fatal(err)
	}
	sql, params := spec.ParentFilter("p1")
	if sql != "`key` = @parent" || params["parent"] != "p1" {
		t.Fatalf("%q %v", sql, params)
	}
}

// TestKeySpecForRejectsUntagged checks that KeySpecFor errors on a struct with an untagged exported field.
func TestKeySpecForRejectsUntagged(t *testing.T) {
	if _, err := KeySpecFor[badKeyWrap](); err == nil {
		t.Fatal("untagged exported field must error")
	}
}

// TestKeySpecForRejectsUnsupportedFieldType checks that KeySpecFor errors on a tagged field of an
// unsupported type ([]byte).
func TestKeySpecForRejectsUnsupportedFieldType(t *testing.T) {
	if _, err := KeySpecFor[unsupportedKey](); err == nil {
		t.Fatal("unsupported field type must error")
	}
}

// TestKeySpecForRejectsMultiColumnParent checks that KeySpecFor errors at construction when
// WithParentColumns asks for more than one parent column.
func TestKeySpecForRejectsMultiColumnParent(t *testing.T) {
	if _, err := KeySpecFor[sessionKey](WithParentColumns(2)); err == nil {
		t.Fatal("multi-column parent must error at construction")
	}
}

// TestKeySpecForRejectsZeroOrNegativeParentColumns checks that KeySpecFor errors at construction
// when WithParentColumns is given zero or a negative count.
func TestKeySpecForRejectsZeroOrNegativeParentColumns(t *testing.T) {
	for _, n := range []int{0, -1} {
		if _, err := KeySpecFor[sessionKey](WithParentColumns(n)); err == nil {
			t.Fatalf("WithParentColumns(%d) must error at construction", n)
		}
	}
}

// TestKeyValuesOfSkipsUnexportedTaggedField checks that KeyValuesOf ignores an unexported field
// carrying a pdb tag instead of panicking on it.
func TestKeyValuesOfSkipsUnexportedTaggedField(t *testing.T) {
	k := unexportedTaggedKey{Public: "pub", private: "priv"}
	got := k.KeyValues() // must not panic despite the stray tag on `private`
	if !reflect.DeepEqual(got, []any{"pub"}) {
		t.Fatal(got)
	}
}

// TestStringKeySpec checks that StringKeySpec has the single named column and that its parent filter
// is a STARTS_WITH prefix match, or no filter for an empty parent.
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

// TestStringKeyImplementsProtodbKey checks that StringKey satisfies protodb.Key and returns its string
// as the single key value.
func TestStringKeyImplementsProtodbKey(t *testing.T) {
	var _ protodb.Key = StringKey("resources/1")
	k := StringKey("resources/1")
	if !reflect.DeepEqual(k.KeyValues(), []any{"resources/1"}) {
		t.Fatal(k.KeyValues())
	}
}

// TestEncodeInjective checks that Encode gives different strings for keys whose field values
// differ only in where a separator-like character falls.
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

// TestEncodeEqualForEqualKeys checks that Encode is deterministic: equal keys encode to the same string.
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

// TestKeySpecForDerefsPointerKind checks that KeySpecFor accepts a pointer-to-struct key type and
// reads the columns of the pointed-to struct.
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

// TestToKey checks that ToKey converts a key's values, in order, into a spanner.Key.
func TestToKey(t *testing.T) {
	got := ToKey(sessionKey{"s", "a", "u"})
	want := spanner.Key{"s", "a", "u"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// TestToKeySets checks that ToKeySets builds a KeySet holding one spanner.Key per protodb.Key.
func TestToKeySets(t *testing.T) {
	keys := []protodb.Key{StringKey("a"), StringKey("b")}
	got := ToKeySets(keys)
	want := spanner.KeySets(spanner.Key{"a"}, spanner.Key{"b"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// TestToKeySetsEmpty checks that ToKeySets on nil returns an empty KeySet rather than panicking.
func TestToKeySetsEmpty(t *testing.T) {
	got := ToKeySets(nil)
	want := spanner.KeySets()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
