package filtering

import "testing"

func TestSanitizeNoSpaceEquals(t *testing.T) {
	p, _ := NewParser()
	stmt, err := p.Parse("app_name='x'")
	if err != nil {
		t.Fatalf("no-space '=' must parse: %v", err)
	}
	if stmt.SQL != "app_name = @p0" {
		t.Fatalf("got %q", stmt.SQL)
	}
}

func TestSanitizePreservesLiterals(t *testing.T) {
	p, _ := NewParser()
	for _, tc := range []struct{ filter, wantParam string }{
		{"name == 'BRAND AND CO'", "BRAND AND CO"}, // AND inside literal
		{"name == 'a=b'", "a=b"},                   // = inside literal
		{"name == 'NULL OR IN'", "NULL OR IN"},     // all keywords inside literal
		{`name == "BRAND AND CO"`, "BRAND AND CO"}, // double-quoted literal
	} {
		stmt, err := p.Parse(tc.filter)
		if err != nil {
			t.Fatalf("%q: %v", tc.filter, err)
		}
		if got := stmt.Params["p0"]; got != tc.wantParam {
			t.Fatalf("%q: param %q want %q", tc.filter, got, tc.wantParam)
		}
	}
}

func TestSanitizeStillHandlesOperators(t *testing.T) {
	p, _ := NewParser()
	for _, f := range []string{"a >= 1", "a <= 1", "a != 'x'", "a == 'x'"} {
		if _, err := p.Parse(f); err != nil {
			t.Fatalf("%q: %v", f, err)
		}
	}
}
