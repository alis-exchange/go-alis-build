package adk

import (
	"testing"
)

func FuzzDecodeRunEvalResults(f *testing.F) {
	f.Add([]byte("[]"))
	f.Add([]byte(`{"runEvalResults":[]}`))
	f.Add([]byte(`[{"evalId":"case-1"}]`))
	f.Add([]byte("not json"))

	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = decodeRunEvalResults(raw)
	})
}
