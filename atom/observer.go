package atom

import (
	"context"
	"time"
)

// Observer is an interface for observing transaction lifecycle events
// Implementations can be used for metrics, tracing, logging, or other observability needs
type Observer interface {
	// OnOperationStart is called before an operation begins execution
	OnOperationStart(ctx context.Context, name string)

	// OnOperationEnd is called after an operation completes (success or failure)
	OnOperationEnd(ctx context.Context, name string, duration time.Duration, err error)

	// OnCommit is called after a successful commit
	OnCommit(ctx context.Context)

	// OnRollback is called after a rollback completes
	// errors contains any errors that occurred during compensation
	OnRollback(ctx context.Context, errors []error)
}

// SetObserver sets the observer for this transaction
// Only one observer can be set at a time; setting a new observer replaces the previous one
func (tx *Transaction) SetObserver(obs Observer) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.observer = obs
}

// GetObserver returns the current observer, if any
func (tx *Transaction) GetObserver() Observer {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.observer
}

// NoOpObserver is an Observer implementation that does nothing
// Useful as a default or for testing
type NoOpObserver struct{}

func (NoOpObserver) OnOperationStart(ctx context.Context, name string)                          {}
func (NoOpObserver) OnOperationEnd(ctx context.Context, name string, duration time.Duration, err error) {}
func (NoOpObserver) OnCommit(ctx context.Context)                                                       {}
func (NoOpObserver) OnRollback(ctx context.Context, errors []error)                                     {}

// LoggingObserver is an Observer implementation that logs events using alog
type LoggingObserver struct{}

// NewLoggingObserver creates a new LoggingObserver
func NewLoggingObserver() *LoggingObserver {
	return &LoggingObserver{}
}

func (l *LoggingObserver) OnOperationStart(ctx context.Context, name string) {
	// Intentionally minimal - detailed logging would use alog
}

func (l *LoggingObserver) OnOperationEnd(ctx context.Context, name string, duration time.Duration, err error) {
	// Intentionally minimal - detailed logging would use alog
}

func (l *LoggingObserver) OnCommit(ctx context.Context) {
	// Intentionally minimal - detailed logging would use alog
}

func (l *LoggingObserver) OnRollback(ctx context.Context, errors []error) {
	// Intentionally minimal - detailed logging would use alog
}

// MetricsObserver is a sample Observer that collects basic metrics
// This is a reference implementation; production use would integrate with
// actual metrics systems like Prometheus, OpenTelemetry, etc.
type MetricsObserver struct {
	OperationCount    int64
	SuccessCount      int64
	FailureCount      int64
	TotalDuration     time.Duration
	CommitCount       int64
	RollbackCount     int64
	RollbackErrorCount int64
}

// NewMetricsObserver creates a new MetricsObserver
func NewMetricsObserver() *MetricsObserver {
	return &MetricsObserver{}
}

func (m *MetricsObserver) OnOperationStart(ctx context.Context, name string) {
	m.OperationCount++
}

func (m *MetricsObserver) OnOperationEnd(ctx context.Context, name string, duration time.Duration, err error) {
	m.TotalDuration += duration
	if err != nil {
		m.FailureCount++
	} else {
		m.SuccessCount++
	}
}

func (m *MetricsObserver) OnCommit(ctx context.Context) {
	m.CommitCount++
}

func (m *MetricsObserver) OnRollback(ctx context.Context, errors []error) {
	m.RollbackCount++
	m.RollbackErrorCount += int64(len(errors))
}
