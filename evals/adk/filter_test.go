package adk

import (
	"testing"

	"go.alis.build/evals/suite"
)

func TestMatchFilters_empty(t *testing.T) {
	t.Parallel()

	wantAll, caseIDs, mentioned := matchFilters(nil, "eval_set_1")
	if !wantAll || !mentioned || len(caseIDs) != 0 {
		t.Fatalf("got wantAll=%v mentioned=%v caseIDs=%v", wantAll, mentioned, caseIDs)
	}
}

func TestMatchFilters_wholeSuite(t *testing.T) {
	t.Parallel()

	parsed, err := suite.ParseFilterPaths([]string{"eval_set_1"})
	if err != nil {
		t.Fatalf("ParseFilterPaths: %v", err)
	}
	wantAll, caseIDs, mentioned := matchFilters(parsed, "eval_set_1")
	if !wantAll || !mentioned || len(caseIDs) != 0 {
		t.Fatalf("got wantAll=%v mentioned=%v caseIDs=%v", wantAll, mentioned, caseIDs)
	}
}

func TestMatchFilters_singleCase(t *testing.T) {
	t.Parallel()

	parsed, err := suite.ParseFilterPaths([]string{"eval_set_1.hi"})
	if err != nil {
		t.Fatalf("ParseFilterPaths: %v", err)
	}
	wantAll, caseIDs, mentioned := matchFilters(parsed, "eval_set_1")
	if wantAll || !mentioned || len(caseIDs) != 1 || caseIDs[0] != "hi" {
		t.Fatalf("got wantAll=%v mentioned=%v caseIDs=%v", wantAll, mentioned, caseIDs)
	}
}

func TestMatchFilters_unmentionedSuite(t *testing.T) {
	t.Parallel()

	parsed, err := suite.ParseFilterPaths([]string{"other.hi"})
	if err != nil {
		t.Fatalf("ParseFilterPaths: %v", err)
	}
	wantAll, caseIDs, mentioned := matchFilters(parsed, "eval_set_1")
	if mentioned || wantAll || len(caseIDs) != 0 {
		t.Fatalf("got wantAll=%v mentioned=%v caseIDs=%v", wantAll, mentioned, caseIDs)
	}
}
