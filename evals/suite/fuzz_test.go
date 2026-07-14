package suite

import (
	"errors"
	"strings"
	"testing"
)

func FuzzParseFilterPath(f *testing.F) {
	for _, path := range []string{
		"suite",
		"suite.case",
		"a.b.c",
		".case",
		"suite.",
		"",
		"my_suite.case_1",
	} {
		f.Add(path)
	}

	f.Fuzz(func(t *testing.T, path string) {
		fp, err := parseFilterPath(path)
		if err != nil {
			if !errors.Is(err, ErrInvalidFilterPath{}) {
				t.Fatalf("parseFilterPath(%q): unexpected error %v", path, err)
			}
			return
		}
		if fp.Suite == "" {
			t.Fatalf("parseFilterPath(%q): empty suite on success", path)
		}
		if strings.Count(path, ".") > 1 {
			t.Fatalf("parseFilterPath(%q): succeeded with multiple dots", path)
		}
		if strings.Contains(path, ".") {
			_, caseName, _ := strings.Cut(path, ".")
			if caseName == "" {
				t.Fatalf("parseFilterPath(%q): succeeded with empty case name", path)
			}
		}

		got, err := ParseFilterPaths([]string{path})
		if err != nil {
			t.Fatalf("ParseFilterPaths(%q): %v", path, err)
		}
		if len(got) != 1 || got[0] != fp {
			t.Fatalf("ParseFilterPaths(%q) = %+v, want %+v", path, got, fp)
		}
	})
}
