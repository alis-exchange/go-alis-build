package loadinfra

// TargetRole classifies an infrastructure target within a case.
type TargetRole int

const (
	// RoleEntry is the primary service receiving traffic or acting as the
	// case entrypoint.
	RoleEntry TargetRole = 1 + iota
	// RoleDependency is a downstream service or datastore observed alongside
	// the entry target.
	RoleDependency
)

// CloudRunTarget declares a Cloud Run service revision scope for Monitoring
// queries.
type CloudRunTarget struct {
	// ID is the stable target identifier from suite configuration (for example
	// `search-v1`). Must be unique across all infra targets in a suite.
	ID string
	// Role is ENTRY for the primary service or DEPENDENCY for a downstream hop.
	Role TargetRole
	// ProjectID is the Google Cloud project hosting the Cloud Run service.
	ProjectID string
	// Region is the Cloud Run region (for example `europe-west1`).
	Region string
	// ServiceName is the Cloud Run service name (for example `search-v1`).
	ServiceName string
	// Revision filters to one revision when non-empty; empty aggregates all
	// revisions.
	Revision string
}

// SpannerTarget declares a Spanner instance and database scope for Monitoring
// queries. Role is always DEPENDENCY on the wire.
type SpannerTarget struct {
	// ID is the stable target identifier from suite configuration (for example
	// `orders-db`). Must be unique across all infra targets in a suite.
	ID string
	// ProjectID is the Google Cloud project hosting the Spanner instance.
	ProjectID string
	// InstanceID is the Spanner instance ID (for example `prod-spanner`).
	InstanceID string
	// Location is the Spanner instance location (for example `europe-west1`).
	Location string
	// Database is the database within the instance (for example `orders`).
	Database string
}
