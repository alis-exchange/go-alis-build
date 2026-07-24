// Package loadinfra fetches Cloud Run and Spanner server-side metrics from
// Cloud Monitoring for load-integrated and standalone infra observation runs.
//
// # Windows
//
// An [ObservationWindow] is inclusive-start, exclusive-end (UTC). Load-integrated
// callers derive it from [WindowFromMetrics] (warmup excluded). Standalone
// callers use [WindowLookback] after [SettleDuration] so recently ingested
// data is visible. Load-integrated [Observe] calls extend Monitoring query
// intervals via [CloudRunQueryWindow] and [SpannerQueryWindow]; standalone
// callers pass extendQueryEnd false so queries use the settled window as-is.
//
// # Targets
//
// Declare [CloudRunTarget] and [SpannerTarget] in normal Go code and pass them
// to [Observe] from a load or infra observation case. Cloud Run requires
// exactly one [RoleEntry]; Spanner targets are always DEPENDENCY on the wire.
// Target IDs must be unique across kinds.
//
// # Fetch semantics (v1)
//
// [Observe] fetches all declared targets concurrently (30s per-target timeout).
// Per-target failures are recorded on the snapshot (FetchStatus, FetchMessage);
// they do not fail the parent load or infra-observe case. Partial metric gaps
// within a target still yield OK with a partial-failure message.
//
// # Client injection
//
// Production code constructs a client with [NewMetricClient]. Callers may pass
// the client directly to [Observe] or attach it to context via [WithClient] for
// their own case helper code. Tests inject [FakeMetricClient] at the same
// boundary.
//
// Example (standalone):
//
//	client, _ := loadinfra.NewMetricClient(ctx)
//	defer client.Close()
//	w := loadinfra.WindowLookback(30*time.Minute, time.Now(), loadinfra.SettleDuration(true, true))
//	obs, err := loadinfra.Observe(ctx, client, cloud, spanner, w, false)
package loadinfra
