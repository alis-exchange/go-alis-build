package adk

import "go.alis.build/evals/suite"

// matchFilters returns whether setID is mentioned in filters, whether all cases
// in the set should run, and which case IDs to run when wantAll is false.
func matchFilters(parsed []suite.FilterPath, setID string) (wantAll bool, caseIDs []string, mentioned bool) {
	if len(parsed) == 0 {
		return true, nil, true
	}

	wantAll = false
	mentioned = false
	seen := make(map[string]struct{})

	for _, f := range parsed {
		if f.Suite != setID {
			continue
		}
		mentioned = true
		if f.CaseName == "" {
			wantAll = true
			return true, nil, true
		}
		if _, ok := seen[f.CaseName]; ok {
			continue
		}
		seen[f.CaseName] = struct{}{}
		caseIDs = append(caseIDs, f.CaseName)
	}

	if !mentioned {
		return false, nil, false
	}
	return wantAll, caseIDs, true
}
