package filtering

import (
	"errors"
	"testing"
)

func TestParamBinding(t *testing.T) {
	p, _ := NewParser()
	stmt, err := p.Parse("app_name == param('app') AND user_id == param('user')",
		map[string]any{"app": "chat", "user": "u'; DROP TABLE x;--"})
	if err != nil {
		t.Fatal(err)
	}
	if stmt.SQL != "(app_name = @p0 AND user_id = @p1)" {
		t.Fatalf("got %q", stmt.SQL)
	}
	if stmt.Params["p0"] != "chat" {
		t.Fatalf("p0=%v", stmt.Params["p0"])
	}
	if stmt.Params["p1"] != "u'; DROP TABLE x;--" {
		t.Fatalf("p1=%v", stmt.Params["p1"])
	}
}

func TestParamUnknownName(t *testing.T) {
	p, _ := NewParser()
	_, err := p.Parse("a == param('missing')", map[string]any{})
	var e ErrInvalidFilter
	if !errors.As(err, &e) {
		t.Fatalf("want ErrInvalidFilter, got %v", err)
	}
}

func TestParamWithoutParamsMap(t *testing.T) {
	p, _ := NewParser()
	if _, err := p.Parse("a == param('x')"); err == nil {
		t.Fatal("want error when no params provided")
	}
}

// TestParamInLike guards against a fix-review finding: like()'s second
// argument previously discarded the isFunction flag and unconditionally
// re-wrapped the returned SQL fragment as a fresh bound param. For
// like(name, param('p')) that meant the literal text "@p0" (not the
// caller's value) was bound as a second, orphaning parameter. It must
// instead embed the param()-produced placeholder directly.
func TestParamInLike(t *testing.T) {
	p, _ := NewParser()
	stmt, err := p.Parse("like(name, param('p'))", map[string]any{"p": "Al%"})
	if err != nil {
		t.Fatal(err)
	}
	if stmt.SQL != "name LIKE @p0" {
		t.Fatalf("got %q", stmt.SQL)
	}
	if len(stmt.Params) != 1 {
		t.Fatalf("want exactly 1 bound param, got %d: %v", len(stmt.Params), stmt.Params)
	}
	if stmt.Params["p0"] != "Al%" {
		t.Fatalf("p0=%v", stmt.Params["p0"])
	}
}

// TestParamInPrefix covers the same fix for prefix() (and, by the same code
// path, suffix()).
func TestParamInPrefix(t *testing.T) {
	p, _ := NewParser()
	stmt, err := p.Parse("prefix(name, param('p'))", map[string]any{"p": "Al"})
	if err != nil {
		t.Fatal(err)
	}
	if stmt.SQL != "STARTS_WITH(name, @p0)" {
		t.Fatalf("got %q", stmt.SQL)
	}
	if len(stmt.Params) != 1 {
		t.Fatalf("want exactly 1 bound param, got %d: %v", len(stmt.Params), stmt.Params)
	}
	if stmt.Params["p0"] != "Al" {
		t.Fatalf("p0=%v", stmt.Params["p0"])
	}
}

// TestParamInSuffix covers the same fix for suffix().
func TestParamInSuffix(t *testing.T) {
	p, _ := NewParser()
	stmt, err := p.Parse("suffix(name, param('p'))", map[string]any{"p": "ce"})
	if err != nil {
		t.Fatal(err)
	}
	if stmt.SQL != "ENDS_WITH(name, @p0)" {
		t.Fatalf("got %q", stmt.SQL)
	}
	if len(stmt.Params) != 1 {
		t.Fatalf("want exactly 1 bound param, got %d: %v", len(stmt.Params), stmt.Params)
	}
	if stmt.Params["p0"] != "ce" {
		t.Fatalf("p0=%v", stmt.Params["p0"])
	}
}

// TestParamInListRejected: a list literal is bound as a single array-typed
// Spanner parameter, so a function/placeholder result like param(...) cannot
// be embedded as one element of that array without silently misbinding it
// (the literal text "@p0" would end up as the array element instead of the
// caller's value). The parser rejects this with a clean ErrInvalidFilter
// rather than mis-binding; mixed literal/param lists are not supported.
func TestParamInListRejected(t *testing.T) {
	p, _ := NewParser()
	_, err := p.Parse("name in [param('a'), 'Bob']", map[string]any{"a": "Alice"})
	var e ErrInvalidFilter
	if !errors.As(err, &e) {
		t.Fatalf("want ErrInvalidFilter, got %v", err)
	}
}
