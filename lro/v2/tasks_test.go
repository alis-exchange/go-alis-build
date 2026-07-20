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
	errors   []error
	requests []*cloudtaskspb.CreateTaskRequest
}

func (f *fakeCloudTasksClient) CreateTask(_ context.Context, req *cloudtaskspb.CreateTaskRequest, _ ...gax.CallOption) (*cloudtaskspb.Task, error) {
	f.requests = append(f.requests, req)
	if len(f.errors) == 0 {
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

func TestScheduleCloudTaskDoesNotRetryPermanentErrors(t *testing.T) {
	client := &fakeCloudTasksClient{
		errors: []error{status.Error(codes.InvalidArgument, "invalid task")},
	}
	q := testQueue(client)

	err := q.scheduleCloudTask(context.Background(), "https://example.test/resume", time.Now())
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("scheduleCloudTask() code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
	if got := len(client.requests); got != 1 {
		t.Fatalf("CreateTask calls = %d, want 1", got)
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
	}
}
