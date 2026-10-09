package loadinfratest_test

import (
	"sync"
	"testing"

	"go.alis.build/evals/loadinfra"
	"go.alis.build/evals/loadinfra/loadinfratest"
)

// The alias must keep satisfying loadinfra.MetricClient.
var _ loadinfra.MetricClient = (*loadinfratest.MetricClient)(nil)

// TestMetricClient_closeIsSafeConcurrently checks that concurrent Close calls
// do not race and are all counted.
func TestMetricClient_closeIsSafeConcurrently(t *testing.T) {
	t.Parallel()
	c := &loadinfratest.MetricClient{}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Close()
		}()
	}
	wg.Wait()
	if c.CloseCalls != 8 {
		t.Fatalf("CloseCalls = %d, want 8", c.CloseCalls)
	}
}
