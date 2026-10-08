package lro

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeCloudTasksClient struct {
	// errors are returned in order; a nil entry succeeds. Once drained, every
	// further call returns always (nil means success).
	errors   []error
	always   error
	requests []*cloudtaskspb.CreateTaskRequest
	// deadlines and ctxErrs record, per call, the deadline and error state of
	// the context CreateTask received.
	deadlines []time.Time
	ctxErrs   []error
}

func (f *fakeCloudTasksClient) CreateTask(ctx context.Context, req *cloudtaskspb.CreateTaskRequest, _ ...gax.CallOption) (*cloudtaskspb.Task, error) {
	f.requests = append(f.requests, req)
	deadline, _ := ctx.Deadline()
	f.deadlines = append(f.deadlines, deadline)
	f.ctxErrs = append(f.ctxErrs, ctx.Err())
	if len(f.errors) == 0 {
		if f.always != nil {
			return nil, f.always
		}
		return req.GetTask(), nil
	}
	err := f.errors[0]
	f.errors = f.errors[1:]
	if err != nil {
		return nil, err
	}
	return req.GetTask(), nil
}

func (f *fakeCloudTasksClient) Close() error { return nil }

func TestScheduleCloudTaskRetriesTransientErrors(t *testing.T) {
	client := &fakeCloudTasksClient{
		errors: []error{
			status.Error(codes.Unavailable, "temporarily unavailable"),
			nil,
		},
	}
	q := testQueue(client)

	if err := q.scheduleCloudTask(context.Background(), "https://example.test/resume", time.Now()); err != nil {
		t.Fatalf("scheduleCloudTask() error = %v", err)
	}

	if got := len(client.requests); got != 2 {
		t.Fatalf("CreateTask calls = %d, want 2", got)
	}
	firstName := client.requests[0].GetTask().GetName()
	if firstName == "" {
		t.Fatal("task name is empty")
	}
	if got := client.requests[1].GetTask().GetName(); got != firstName {
		t.Fatalf("retry task name = %q, want %q", got, firstName)
	}
}

func TestScheduleCloudTaskTreatsAlreadyExistsAfterRetryAsSuccess(t *testing.T) {
	client := &fakeCloudTasksClient{
		errors: []error{
			status.Error(codes.Unavailable, "response lost"),
			status.Error(codes.AlreadyExists, "task was created"),
		},
	}
	q := testQueue(client)

	if err := q.scheduleCloudTask(context.Background(), "https://example.test/resume", time.Now()); err != nil {
		t.Fatalf("scheduleCloudTask() error = %v", err)
	}
	if got := len(client.requests); got != 2 {
		t.Fatalf("CreateTask calls = %d, want 2", got)
	}
}

// Cloud Tasks has been seen rejecting a run of identical CreateTask calls with
// InvalidArgument for a few seconds while neighbouring calls succeeded, so
// "permanent" codes are retried too; the budget is what bounds the attempts.
func TestScheduleCloudTaskRetriesInvalidArgument(t *testing.T) {
	client := &fakeCloudTasksClient{
		errors: []error{
			status.Error(codes.InvalidArgument, "Request contains an invalid argument."),
			status.Error(codes.InvalidArgument, "Request contains an invalid argument."),
			nil,
		},
	}
	q := testQueue(client)

	if err := q.scheduleCloudTask(context.Background(), "https://example.test/resume", time.Now()); err != nil {
		t.Fatalf("scheduleCloudTask() error = %v", err)
	}
	if got := len(client.requests); got != 3 {
		t.Fatalf("CreateTask calls = %d, want 3", got)
	}
	for i, req := range client.requests[1:] {
		if got, want := req.GetTask().GetName(), client.requests[0].GetTask().GetName(); got != want {
			t.Fatalf("retry %d task name = %q, want %q", i+1, got, want)
		}
	}
}

func TestScheduleCloudTaskGivesUpWhenBudgetIsExhausted(t *testing.T) {
	client := &fakeCloudTasksClient{
		always: status.Error(codes.InvalidArgument, "Request contains an invalid argument."),
	}
	q := testQueue(client)
	q.scheduleBudget = 20 * time.Millisecond

	err := q.scheduleCloudTask(context.Background(), "https://example.test/resume", time.Now())
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("scheduleCloudTask() code = %v, want %v (err %v)", status.Code(err), codes.InvalidArgument, err)
	}
	if got := len(client.requests); got < 2 {
		t.Fatalf("CreateTask calls = %d, want at least 2", got)
	}
}

// The caller's context must decide nothing: a long RPC deadline would be
// forwarded to Cloud Tasks and rejected ("The deadline cannot be more than 30s
// in the future"), and a disconnected caller would cancel the schedule.
func TestScheduleCloudTaskDetachesFromCallerContext(t *testing.T) {
	client := &fakeCloudTasksClient{}
	q := testQueue(client)

	parent, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	cancel()

	before := time.Now()
	if err := q.scheduleCloudTask(parent, "https://example.test/resume", time.Now()); err != nil {
		t.Fatalf("scheduleCloudTask() error = %v", err)
	}
	if got := len(client.requests); got != 1 {
		t.Fatalf("CreateTask calls = %d, want 1", got)
	}
	if err := client.ctxErrs[0]; err != nil {
		t.Fatalf("CreateTask context error = %v, want nil despite the cancelled caller", err)
	}
	deadline := client.deadlines[0]
	if deadline.IsZero() {
		t.Fatal("CreateTask context has no deadline")
	}
	if latest := before.Add(q.createTaskTimeout + time.Second); deadline.After(latest) {
		t.Fatalf("CreateTask deadline %s is later than %s", deadline, latest)
	}
}

func testQueue(client cloudTasksClient) *queue {
	return &queue{
		name:                "projects/test/locations/europe-west1/queues/test-operations",
		serviceAccountEmail: "alis-build@test.iam.gserviceaccount.com",
		client:              client,
		taskDeadline:        time.Minute,
		retryBackoff: gax.Backoff{
			Initial:    time.Nanosecond,
			Max:        time.Nanosecond,
			Multiplier: 2,
		},
		createTaskTimeout: defaultCreateTaskTimeout,
		scheduleBudget:    time.Second,
	}
}
