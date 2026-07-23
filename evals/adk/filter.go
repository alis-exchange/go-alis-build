package adk

import (
	"fmt"
	"strings"
)

// filterPath is a parsed case filter ("suite" or "suite.case").
type filterPath struct {
	suite    string
	caseName string // empty means whole suite
}

func parseFilterPaths(paths []string) ([]filterPath, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	out := make([]filterPath, len(paths))
	for i, path := range paths {
		parsed, err := parseFilterPath(path)
		if err != nil {
			return nil, err
		}
		out[i] = parsed
	}
	return out, nil
}

func parseFilterPath(path string) (filterPath, error) {
	if path == "" {
		return filterPath{}, fmt.Errorf("adk: invalid filter path %q: empty filter path", path)
	}
	if strings.Count(path, ".") > 1 {
		return filterPath{}, fmt.Errorf("adk: invalid filter path %q: at most one '.' allowed", path)
	}
	suiteName, caseName, hasCase := strings.Cut(path, ".")
	if suiteName == "" || hasCase && caseName == "" {
		return filterPath{}, fmt.Errorf("adk: invalid filter path %q", path)
	}
	return filterPath{suite: suiteName, caseName: caseName}, nil
}

// matchFilters returns whether setID is mentioned in filters, whether all cases
// in the set should run, and which case IDs to run when wantAll is false.
func matchFilters(parsed []filterPath, setID string) (wantAll bool, caseIDs []string, mentioned bool) {
	if len(parsed) == 0 {
		return true, nil, true
	}

	wantAll = false
	mentioned = false
	seen := make(map[string]struct{})

	for _, f := range parsed {
		if f.suite != setID {
			continue
		}
		mentioned = true
		if f.caseName == "" {
			wantAll = true
			return true, nil, true
		}
		if _, ok := seen[f.caseName]; ok {
			continue
		}
		seen[f.caseName] = struct{}{}
		caseIDs = append(caseIDs, f.caseName)
	}

	if !mentioned {
		return false, nil, false
	}
	return wantAll, caseIDs, true
}
