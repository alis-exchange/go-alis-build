package pubsub

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/events"
	"google.golang.org/protobuf/proto"
)

// setEmulator points the underlying pubsub client at a non-routable
// emulator address so real events.NewClient calls skip Application
// Default Credentials and the GCE metadata server. No emulator needs to
// be running; nothing here publishes.
func setEmulator(t *testing.T) {
	t.Helper()
	t.Setenv("PUBSUB_EMULATOR_HOST", "127.0.0.1:0")
}

func TestReporter_ReportRun_nilSafe(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var r *Reporter
		if err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/1"}); err != nil {
			t.Errorf("ReportRun on nil reporter err = %v, want nil", err)
		}
	})

	t.Run("nil run", func(t *testing.T) {
		fake := &recordingPublisher{}
		r := newReporterWithPublisher(fake)
		if err := r.ReportRun(context.Background(), nil); err != nil {
			t.Errorf("ReportRun with nil run err = %v, want nil", err)
		}
		if fake.publishCount() != 0 {
			t.Errorf("Publish call count = %d, want 0 (nil run should short-circuit)", fake.publishCount())
		}
	})

	t.Run("zero-value receiver (nil publisher)", func(t *testing.T) {
		// `&Reporter{}` compiles because the type is exported. Guarding
		// against a nil publisher keeps ReportRun from panicking in
		// production if a misuse ever slips through.
		r := &Reporter{}
		if err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/1"}); err != nil {
			t.Errorf("ReportRun on zero-value reporter err = %v, want nil", err)
		}
	})
}

func TestReporter_ReportRun_wrapsInRunPublishedEvent(t *testing.T) {
	fake := &recordingPublisher{}
	r := newReporterWithPublisher(fake)

	run := &evalspb.Run{Name: "runs/wrap-me"}
	if err := r.ReportRun(context.Background(), run); err != nil {
		t.Fatalf("ReportRun err = %v, want nil", err)
	}

	evt, ok := fake.lastMessage().(*evalspb.RunPublishedEvent)
	if !ok {
		t.Fatalf("published message type = %T, want *evalspb.RunPublishedEvent", fake.lastMessage())
	}
	if evt.GetRun() != run {
		t.Errorf("event.Run pointer = %p, want %p", evt.GetRun(), run)
	}
}

func TestReporter_ReportRun_forwardsPublishOptions(t *testing.T) {
	fake := &recordingPublisher{}
	r := newReporterWithPublisher(
		fake,
		WithTopic("custom.topic"),
		WithOrderingKey("shard-42"),
		WithBackground(),
	)

	// Construction assertion: the reporter should have built a
	// PublishOption for each non-empty setting.
	if got, want := len(r.publishOpts), 3; got != want {
		t.Fatalf("built publishOpts count = %d, want %d", got, want)
	}

	if err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/opts"}); err != nil {
		t.Fatalf("ReportRun err = %v, want nil", err)
	}

	// Forwarding assertion: whatever the reporter built should be
	// forwarded verbatim to Publish. We can't inspect the closure
	// contents (events.PublishOptions has unexported fields), so we
	// assert the count matches. Combined with per-option construction
	// tests below, this proves the plumbing is complete.
	if got, want := len(fake.lastOpts()), len(r.publishOpts); got != want {
		t.Errorf("forwarded option count = %d, want %d", got, want)
	}
}

func TestReporter_construction_publishOpts(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want int
	}{
		{"no options → no publish opts", nil, 0},
		{"WithTopic adds one", []Option{WithTopic("t")}, 1},
		{"WithOrderingKey adds one", []Option{WithOrderingKey("k")}, 1},
		{"WithBackground adds one", []Option{WithBackground()}, 1},
		{"empty WithTopic drops the option", []Option{WithTopic("")}, 0},
		{"empty WithOrderingKey drops the option", []Option{WithOrderingKey("")}, 0},
		{"all three combine", []Option{WithTopic("t"), WithOrderingKey("k"), WithBackground()}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReporterWithPublisher(&recordingPublisher{}, tt.opts...)
			if got := len(r.publishOpts); got != tt.want {
				t.Errorf("publishOpts count = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestReporter_ReportRun_wrapsPublishError(t *testing.T) {
	sentinel := errors.New("boom")
	fake := &recordingPublisher{err: sentinel}
	r := newReporterWithPublisher(fake)

	err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/boom"})
	if err == nil {
		t.Fatal("ReportRun err = nil, want wrapped error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("errors.Is(err, sentinel) = false, want true; err = %v", err)
	}
}

func TestReporter_ReportRun_timeoutHonored(t *testing.T) {
	fake := &blockingPublisher{released: make(chan struct{})}
	defer close(fake.released)

	r := newReporterWithPublisher(fake, WithPublishTimeout(10*time.Millisecond))

	start := time.Now()
	err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/slow"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("ReportRun err = nil, want deadline error")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("ReportRun took %v, want <500ms (timeout should short-circuit)", elapsed)
	}
	ctxErr := fake.observedCtxErr()
	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Errorf("observed ctx err = %v, want context.DeadlineExceeded", ctxErr)
	}
}

func TestReporter_ReportRun_parentContextCancels(t *testing.T) {
	fake := &blockingPublisher{released: make(chan struct{})}
	defer close(fake.released)

	r := newReporterWithPublisher(fake, WithPublishTimeout(time.Hour))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.ReportRun(ctx, &evalspb.Run{Name: "runs/parent-cancel"})
	if err == nil {
		t.Fatal("ReportRun err = nil, want cancellation error")
	}
	ctxErr := fake.observedCtxErr()
	if !errors.Is(ctxErr, context.Canceled) {
		t.Errorf("observed ctx err = %v, want context.Canceled", ctxErr)
	}
}

func TestReporter_Close_borrowedClient(t *testing.T) {
	fake := &recordingPublisher{}
	r := newReporterWithPublisher(fake)

	if err := r.Close(); err != nil {
		t.Errorf("Close on borrowed reporter err = %v, want nil", err)
	}
	if fake.closeCount() != 0 {
		t.Errorf("Close forwarded to publisher %d times, want 0 (borrowed client is caller's responsibility)", fake.closeCount())
	}
}

func TestReporter_Close_ownedClient(t *testing.T) {
	fake := &recordingPublisher{}
	r := newReporterWithPublisher(fake)
	r.closer = fake.Close

	if err := r.Close(); err != nil {
		t.Errorf("Close on owned reporter err = %v, want nil", err)
	}
	if got := fake.closeCount(); got != 1 {
		t.Errorf("publisher.Close call count = %d, want 1", got)
	}
}

func TestReporter_Close_idempotent(t *testing.T) {
	fake := &recordingPublisher{}
	r := newReporterWithPublisher(fake)
	r.closer = fake.Close

	if err := r.Close(); err != nil {
		t.Fatalf("first Close err = %v, want nil", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("second Close err = %v, want nil", err)
	}
	if got := fake.closeCount(); got != 1 {
		t.Errorf("publisher.Close call count = %d, want 1 (Close must be idempotent)", got)
	}
}

func TestReporter_Close_propagatesError(t *testing.T) {
	sentinel := errors.New("close-failed")
	fake := &recordingPublisher{closeErr: sentinel}
	r := newReporterWithPublisher(fake)
	r.closer = fake.Close

	if err := r.Close(); !errors.Is(err, sentinel) {
		t.Errorf("Close err = %v, want %v", err, sentinel)
	}
}

func TestReporter_Close_nilReceiver(t *testing.T) {
	var r *Reporter
	if err := r.Close(); err != nil {
		t.Errorf("Close on nil reporter err = %v, want nil", err)
	}
}

func TestNewWithClient_validation(t *testing.T) {
	t.Run("nil client errors", func(t *testing.T) {
		got, err := NewWithClient(nil)
		if err == nil {
			t.Fatal("NewWithClient(nil) err = nil, want error")
		}
		if got != nil {
			t.Errorf("NewWithClient(nil) reporter = %v, want nil", got)
		}
	})

	t.Run("WithProject rejected", func(t *testing.T) {
		setEmulator(t)
		t.Setenv("ALIS_OS_PROJECT", "test-project")
		client, err := events.NewClient(context.Background())
		if err != nil {
			t.Fatalf("events.NewClient err = %v, want nil", err)
		}
		t.Cleanup(func() { _ = client.Close() })

		got, err := NewWithClient(client, WithProject("other"))
		if err == nil {
			t.Fatal("NewWithClient with WithProject err = nil, want error")
		}
		if got != nil {
			t.Errorf("NewWithClient with WithProject reporter = %v, want nil", got)
		}
	})

	t.Run("succeeds with valid client", func(t *testing.T) {
		setEmulator(t)
		t.Setenv("ALIS_OS_PROJECT", "test-project")
		client, err := events.NewClient(context.Background())
		if err != nil {
			t.Fatalf("events.NewClient err = %v, want nil", err)
		}
		t.Cleanup(func() { _ = client.Close() })

		r, err := NewWithClient(client)
		if err != nil {
			t.Fatalf("NewWithClient err = %v, want nil", err)
		}
		if r == nil {
			t.Fatal("NewWithClient reporter = nil, want non-nil")
		}
		if r.closer != nil {
			t.Error("NewWithClient should not set closer (client is borrowed)")
		}
	})
}

func TestNew_requiresProject(t *testing.T) {
	setEmulator(t)
	t.Setenv("ALIS_OS_PROJECT", "")

	got, err := New(context.Background())
	if err == nil {
		t.Fatal("New() err = nil, want error when no project is available")
	}
	if got != nil {
		t.Errorf("New() reporter = %v, want nil", got)
	}
}

func TestNew_ownsClient(t *testing.T) {
	setEmulator(t)
	t.Setenv("ALIS_OS_PROJECT", "test-project")

	r, err := New(context.Background())
	if err != nil {
		t.Fatalf("New() err = %v, want nil", err)
	}
	if r.closer == nil {
		t.Error("New() should set closer (client is owned)")
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close on owned reporter err = %v, want nil", err)
	}
}

func TestLoadConfig_publishTimeoutDefault(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want time.Duration
	}{
		{"default", nil, defaultPublishTimeout},
		{"explicit positive", []Option{WithPublishTimeout(3 * time.Second)}, 3 * time.Second},
		{"zero snaps to default", []Option{WithPublishTimeout(0)}, defaultPublishTimeout},
		{"negative snaps to default", []Option{WithPublishTimeout(-1)}, defaultPublishTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := loadConfig(tt.opts)
			if cfg.publishTimeout != tt.want {
				t.Errorf("publishTimeout = %v, want %v", cfg.publishTimeout, tt.want)
			}
		})
	}
}

// recordingPublisher captures Publish and Close calls without doing I/O.
type recordingPublisher struct {
	mu        sync.Mutex
	msgs      []proto.Message
	optsCalls [][]events.PublishOption
	err       error
	closes    int
	closeErr  error
}

func (p *recordingPublisher) Publish(ctx context.Context, event proto.Message, opts ...events.PublishOption) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msgs = append(p.msgs, event)
	p.optsCalls = append(p.optsCalls, opts)
	return p.err
}

func (p *recordingPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closes++
	return p.closeErr
}

func (p *recordingPublisher) publishCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.msgs)
}

func (p *recordingPublisher) lastMessage() proto.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.msgs) == 0 {
		return nil
	}
	return p.msgs[len(p.msgs)-1]
}

func (p *recordingPublisher) lastOpts() []events.PublishOption {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.optsCalls) == 0 {
		return nil
	}
	return p.optsCalls[len(p.optsCalls)-1]
}

func (p *recordingPublisher) closeCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closes
}

// blockingPublisher blocks Publish until released is closed or the ctx
// fires. It records the ctx.Err() observed at unblock so tests can assert
// timeouts and cancellations propagated correctly.
type blockingPublisher struct {
	mu       sync.Mutex
	ctxErr   error
	released chan struct{}
}

func (p *blockingPublisher) Publish(ctx context.Context, event proto.Message, opts ...events.PublishOption) error {
	select {
	case <-ctx.Done():
		p.mu.Lock()
		p.ctxErr = ctx.Err()
		p.mu.Unlock()
		return ctx.Err()
	case <-p.released:
		return nil
	}
}

func (p *blockingPublisher) Close() error { return nil }

func (p *blockingPublisher) observedCtxErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ctxErr
}
