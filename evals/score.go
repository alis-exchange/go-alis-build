package evals

import "strings"

// Rouge1F1 returns the ROUGE-1 unigram F1 score between hypothesis and
// reference. Empty inputs are handled: two empty strings score 1; one empty
// side scores 0. Callers decide how to validate or record the score.
func Rouge1F1(hypothesis, reference string) float64 {
	refTokens := rougeTokenize(reference)
	hypTokens := rougeTokenize(hypothesis)
	if len(refTokens) == 0 && len(hypTokens) == 0 {
		return 1
	}
	if len(refTokens) == 0 || len(hypTokens) == 0 {
		return 0
	}
	overlap := rougeUnigramOverlap(hypTokens, refTokens)
	precision := float64(overlap) / float64(len(hypTokens))
	recall := float64(overlap) / float64(len(refTokens))
	if precision+recall == 0 {
		return 0
	}
	return 2 * precision * recall / (precision + recall)
}

// rougeTokenize lowercases and splits on whitespace for ROUGE-1 scoring.
func rougeTokenize(s string) []string {
	fields := strings.Fields(strings.ToLower(s))
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// rougeUnigramOverlap counts matched unigrams with multiset semantics.
func rougeUnigramOverlap(hyp, ref []string) int {
	counts := make(map[string]int, len(ref))
	for _, t := range ref {
		counts[t]++
	}
	overlap := 0
	for _, t := range hyp {
		if counts[t] > 0 {
			counts[t]--
			overlap++
		}
	}
	return overlap
}
