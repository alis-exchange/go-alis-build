package loadgen

import (
	"errors"
	"math"
	"testing"
	"time"
)

func FuzzAggregatorRecordFinalize(f *testing.F) {
	f.Add(int64(5000), true, true, int32(3), uint8(10))
	f.Add(int64(1000), false, false, int32(0), uint8(5))

	f.Fuzz(func(t *testing.T, latencyUs int64, hasTransportErr bool, hasStream bool, messages int32, sampleCount uint8) {
		if latencyUs < 0 || latencyUs > math.MaxInt64/int64(time.Microsecond) {
			return
		}
		if messages < 0 {
			return
		}
		n := int(sampleCount % 50)
		if n == 0 {
			n = 1
		}

		start := time.Now().Add(-time.Minute)
		agg := newAggregator(start, start.Add(time.Minute), 30*time.Second, 0)

		var transportErr error
		if hasTransportErr {
			transportErr = errors.New("UNAVAILABLE")
		}

		for i := 0; i < n; i++ {
			var stream *StreamSample
			if hasStream {
				stream = &StreamSample{
					SendDuration:    time.Duration(latencyUs) * time.Microsecond,
					ResponseLatency: time.Duration(latencyUs/2) * time.Microsecond,
					TotalDuration:   time.Duration(latencyUs) * time.Microsecond,
					MessagesSent:    int(messages),
				}
			}
			agg.record(sample{
				sentAt:  start.Add(time.Duration(i) * time.Millisecond),
				latency: time.Duration(latencyUs) * time.Microsecond,
				result: TargetResult{
					TransportErr: transportErr,
					Stream:       stream,
				},
			})
		}

		final := agg.finalize()
		abort := agg.abortSnapshot()

		if final.RequestCount != int64(n) {
			t.Fatalf("RequestCount=%d, want %d", final.RequestCount, n)
		}
		if final.ErrorCount != abort.ErrorCount {
			t.Fatalf("ErrorCount mismatch: final=%d abort=%d", final.ErrorCount, abort.ErrorCount)
		}
		if final.ActualQPS != abort.ActualQPS {
			t.Fatalf("ActualQPS mismatch: final=%v abort=%v", final.ActualQPS, abort.ActualQPS)
		}
		if final.Latency.P50Ms != abort.Latency.P50Ms ||
			final.Latency.P95Ms != abort.Latency.P95Ms ||
			final.Latency.P99Ms != abort.Latency.P99Ms {
			t.Fatalf("latency percentiles mismatch: final=%+v abort=%+v", final.Latency, abort.Latency)
		}
		if final.Latency.P50Ms > final.Latency.P95Ms+1e-9 || final.Latency.P95Ms > final.Latency.P99Ms+1e-9 {
			t.Fatalf("percentile ordering violated: %+v", final.Latency)
		}
		if hasTransportErr {
			if final.ErrorCount != int64(n) {
				t.Fatalf("ErrorCount=%d, want %d", final.ErrorCount, n)
			}
			if len(final.ErrorsByCode) == 0 {
				t.Fatal("ErrorsByCode empty")
			}
		}
		if hasStream {
			if final.Stream == nil || abort.Stream == nil {
				t.Fatal("stream summary missing")
			}
			if final.Stream.StreamCount != int64(n) {
				t.Fatalf("StreamCount=%d, want %d", final.Stream.StreamCount, n)
			}
			if abort.Stream.TTFB.P99Ms != final.Stream.TTFB.P99Ms {
				t.Fatalf("TTFB P99 mismatch: abort=%v final=%v", abort.Stream.TTFB.P99Ms, final.Stream.TTFB.P99Ms)
			}
		}
		if abort.ErrorsByCode != nil {
			t.Fatal("abort snapshot should omit ErrorsByCode")
		}
	})
}
