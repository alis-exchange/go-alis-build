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
