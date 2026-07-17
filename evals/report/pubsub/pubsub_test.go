package pubsub

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
			t.Errorf("Publish call count = %d, want 0", fake.publishCount())
		}
	})
	t.Run("zero-value receiver", func(t *testing.T) {
		r := &Reporter{}
		if err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/1"}); err != nil {
			t.Errorf("ReportRun on zero-value reporter err = %v, want nil", err)
		}
	})
}

func TestReporter_ReportRun_marshalsJSONMatchesGolden(t *testing.T) {
	start := timestamppb.New(time.Unix(1700000000, 0))
	end := timestamppb.New(time.Unix(1700000005, 0))
	tests := []struct {
		name    string
		fixture string
		run     *evalspb.Run
	}{
		{
			name:    "integration_test",
			fixture: "run.integration.json",
			run: &evalspb.Run{
				Name: "runs/abc", Type: evalspb.Run_INTEGRATION_TEST, Status: evalspb.Status_PASSED,
				StartTime: start, EndTime: end,
				Data: &evalspb.Run_IntegrationTest{
					IntegrationTest: &evalspb.IntegrationTestResults{
						Cases: []*evalspb.IntegrationTestResults_Case{
							{Id: "case-1", Status: evalspb.Status_PASSED, Duration: durationpb.New(1500 * time.Millisecond)},
						},
					},
				},
			},
		},
		{
			name:    "load_test",
			fixture: "run.load.json",
			run: &evalspb.Run{
				Name: "runs/load", Type: evalspb.Run_LOAD_TEST, Status: evalspb.Status_PASSED,
				StartTime: start, EndTime: end,
				Data: &evalspb.Run_LoadTest{
					LoadTest: &evalspb.LoadTestResults{
						Cases: []*evalspb.LoadTestResults_Case{
							{Id: "load-1", Status: evalspb.Status_PASSED},
						},
					},
				},
			},
		},
		{
			name:    "agent_eval",
			fixture: "run.agent.json",
			run: &evalspb.Run{
				Name: "runs/agent", Type: evalspb.Run_AGENT_EVAL, Status: evalspb.Status_PASSED,
				StartTime: start, EndTime: end,
				Data: &evalspb.Run_AgentEval{
					AgentEval: &evalspb.AgentEvalResults{
						Cases: []*evalspb.AgentEvalResults_Case{
							{Id: "agent-1", Status: evalspb.Status_PASSED},
						},
					},
				},
			},
		},
		{
			name:    "infra_observation",
			fixture: "run.infra_observation.json",
			run: &evalspb.Run{
				Name: "runs/infra", Type: evalspb.Run_INFRA_OBSERVATION, Status: evalspb.Status_PASSED,
				StartTime: start, EndTime: end,
				Data: &evalspb.Run_InfraObservation{
					InfraObservation: &evalspb.InfraObservationResults{
						Cases: []*evalspb.InfraObservationResults_Case{
							{
								Id: "peak.hourly", Status: evalspb.Status_PASSED,
								Lookback: durationpb.New(30 * time.Minute),
								WindowStart: start, WindowEnd: end,
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &recordingPublisher{}
			r := newReporterWithPublisher(fake)
			if err := r.ReportRun(context.Background(), tt.run); err != nil {
				t.Fatalf("ReportRun: %v", err)
			}
			got := fake.lastData()
			want, err := os.ReadFile(filepath.Join("testdata", tt.fixture))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if !jsonPayloadEqual(got, want) {
				t.Fatalf("payload mismatch\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestReporter_ReportRun_loadEntriesAreJSONArray(t *testing.T) {
	t.Parallel()
	run := &evalspb.Run{
		Name: "runs/load-entries", Type: evalspb.Run_LOAD_TEST, Status: evalspb.Status_PASSED,
		Data: &evalspb.Run_LoadTest{
			LoadTest: &evalspb.LoadTestResults{
				Cases: []*evalspb.LoadTestResults_Case{
					{
						Id: "load-1", Status: evalspb.Status_PASSED,
						Tags: []*evalspb.LoadTestResults_StringEntry{
							{Key: "rpc", Value: "ListFiles"},
						},
						Summary: &evalspb.LoadTestResults_Summary{
							ErrorsByCode: []*evalspb.LoadTestResults_Int64Entry{
								{Key: "UNAVAILABLE", Value: 2},
							},
						},
					},
				},
			},
		},
	}
	fake := &recordingPublisher{}
	r := newReporterWithPublisher(fake)
	if err := r.ReportRun(context.Background(), run); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	got := string(fake.lastData())
	for _, fragment := range []string{
		`"tags":[`,
		`"key":"rpc"`,
		`"value":"ListFiles"`,
		`"errors_by_code":[`,
		`"key":"UNAVAILABLE"`,
		`"value":"2"`,
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("payload missing %q\n got: %s", fragment, got)
		}
	}
	// Pub/Sub → BigQuery requires array entries, not protojson map objects.
	if strings.Contains(got, `"tags":{`) || strings.Contains(got, `"errors_by_code":{`) {
		t.Fatalf("payload uses object-form maps; want repeated entry arrays\n got: %s", got)
	}
}

func TestReporter_ReportRun_syncBlocksOnAck(t *testing.T) {
	delay := 50 * time.Millisecond
	fake := &blockingPublisher{delay: delay}
	r := newReporterWithPublisher(fake)
	start := time.Now()
	if err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/sync"}); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	if elapsed := time.Since(start); elapsed < delay {
		t.Fatalf("ReportRun returned in %v, want at least %v", elapsed, delay)
	}
}

func TestReporter_ReportRun_background_returnsImmediately(t *testing.T) {
	fake := &blockingPublisher{delay: time.Hour}
	r := newReporterWithPublisher(fake, WithBackground())
	start := time.Now()
	if err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/bg"}); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("background ReportRun took %v, want immediate return", elapsed)
	}
}

func TestReporter_ReportRun_publishTimeoutHonored(t *testing.T) {
	fake := &blockingPublisher{delay: time.Hour}
	r := newReporterWithPublisher(fake, WithPublishTimeout(30*time.Millisecond))
	err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/timeout"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded in chain", err)
	}
}

func TestReporter_ReportRun_forwardsOrderingKey(t *testing.T) {
	fake := &recordingPublisher{}
	r := newReporterWithPublisher(fake, WithOrderingKey("suite-42"))
	if err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/k"}); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	if got := fake.lastMsg().OrderingKey; got != "suite-42" {
		t.Errorf("OrderingKey = %q, want suite-42", got)
	}
}

func TestReporter_ReportRun_topicInErrors(t *testing.T) {
	fake := &recordingPublisher{err: errors.New("broker down")}
	r := newReporterWithPublisher(fake, WithTopic("custom.topic"))
	err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/err"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "custom.topic") {
		t.Errorf("err = %q, want topic name", err.Error())
	}
}

func TestNewWithClient_rejectsWithProject(t *testing.T) {
	client := &pubsub.Client{}
	_, err := NewWithClient(client, WithProject("other"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNew_fallsBackToAlisOsProject(t *testing.T) {
	t.Setenv(projectEnvVar, "from-env")
	var gotProject string
	orig := newPubsubClient
	newPubsubClient = func(_ context.Context, projectID string) (*pubsub.Client, error) {
		gotProject = projectID
		return &pubsub.Client{}, nil
	}
	t.Cleanup(func() { newPubsubClient = orig })

	r, err := New(context.Background())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if gotProject != "from-env" {
		t.Errorf("project = %q, want from-env", gotProject)
	}
	if r.clientCloser == nil {
		t.Error("New should own the client (clientCloser must be non-nil)")
	}
	r.clientCloser = func() error { return nil }
	_ = r.Close()
}

func jsonPayloadEqual(got, want []byte) bool {
	var g, w evalspb.Run
	if err := protojson.Unmarshal(got, &g); err != nil {
		return false
	}
	if err := protojson.Unmarshal(want, &w); err != nil {
		return false
	}
	return proto.Equal(&g, &w)
}

type recordingPublisher struct {
	mu    sync.Mutex
	msgs  []*pubsub.Message
	err   error
	stopN int
}

func (p *recordingPublisher) Publish(_ context.Context, msg *pubsub.Message) publishResult {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msgs = append(p.msgs, msg)
	return &immediateResult{err: p.err}
}

func (p *recordingPublisher) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopN++
}

func (p *recordingPublisher) publishCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.msgs)
}

func (p *recordingPublisher) lastData() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.msgs) == 0 {
		return nil
	}
	return append([]byte(nil), p.msgs[len(p.msgs)-1].Data...)
}

func (p *recordingPublisher) lastMsg() *pubsub.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.msgs) == 0 {
		return nil
	}
	return p.msgs[len(p.msgs)-1]
}

type immediateResult struct {
	err error
}

func (r *immediateResult) Get(context.Context) (string, error) {
	return "msg-id", r.err
}

type blockingPublisher struct {
	delay time.Duration
}

func (p *blockingPublisher) Publish(ctx context.Context, msg *pubsub.Message) publishResult {
	return &blockingResult{ctx: ctx, delay: p.delay, data: msg.Data}
}

func (p *blockingPublisher) Stop() {}

type blockingResult struct {
	ctx   context.Context
	delay time.Duration
	data  []byte
}

func (r *blockingResult) Get(ctx context.Context) (string, error) {
	timer := time.NewTimer(r.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timer.C:
		return "msg-id", nil
	}
}
