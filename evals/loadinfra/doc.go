// Package loadinfra fetches Cloud Run and Spanner server-side metrics from
// Cloud Monitoring for load-integrated and standalone infra observation runs.
//
// # Observation modes
//
// [ObserveLoad] observes the measurement timestamps in loadgen metrics and
// extends Monitoring queries for ingestion delay. [ObserveLookback] resolves a
// settled standalone window from a duration. Advanced callers use [Observe]
// with a named [Request] for a custom window. An [ObservationWindow] is
// inclusive-start, exclusive-end (UTC).
//
// # Targets
//
// Declare [CloudRunTarget] and [SpannerTarget] in a [Targets] value and pass it
// to an observation function. Cloud Run requires exactly one [RoleEntry];
// Spanner targets are always DEPENDENCY on the wire. Target IDs must be unique
// across kinds.
//
// # Fetch semantics (v1)
//
// Observation fetches all declared targets concurrently (30s per-target
// timeout).
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
//	targets := loadinfra.Targets{CloudRun: cloud, Spanner: spanner}
//	obs, err := loadinfra.ObserveLookback(ctx, client, targets, 30*time.Minute)
package loadinfra
