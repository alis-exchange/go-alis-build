package lro

import (
	context "context"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestOperationResumeViaTasksWithoutHTTPHandlersInvokesHandlerDirectly(t *testing.T) {
	resumed := make(chan *Operation, 1)

	client := &Client{
		host:      "https://example.test",
		muxPrefix: normalizePrefix("/resume-operation/"),
		taskQueue: &queue{
			taskDeadline: time.Second,
		},
		resumableHandlers: &sync.Map{},
	}
	client.resumableHandlers.Store("create-agent", ResumeHandler(func(op *Operation) {
		resumed <- op
	}))

	op := &Operation{
		row: &OperationRow{
			Operation: &longrunningpb.Operation{Name: "operations/test-op"},
		},
		Ctx:    context.Background(),
		client: client,
	}

	if err := op.ResumeViaTasks("create-agent", 0); err != nil {
		t.Fatalf("ResumeViaTasks() error = %v", err)
	}

	select {
	case resumedOp := <-resumed:
		if resumedOp == nil {
			t.Fatal("resumed operation is nil")
		}
		if resumedOp.OperationPb().GetName() != "operations/test-op" {
			t.Fatalf("resumed operation name = %q", resumedOp.OperationPb().GetName())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for direct local resume")
	}
}

// On Cloud Run, a Cloud Tasks rejection must not fail the operation: it
// resumes once in-process at the scheduled time instead and stays alive.
func TestOperationResumeViaTasksFallsBackToLocalResumeWhenCloudTasksRejects(t *testing.T) {
	t.Setenv("K_SERVICE", "dbd-v1")
	resumed := make(chan *Operation, 1)

	tasks := &fakeCloudTasksClient{
		always: status.Error(codes.InvalidArgument, "Request contains an invalid argument."),
	}
	q := testQueue(tasks)
	q.scheduleBudget = 20 * time.Millisecond

	client := &Client{
		host:              "https://example.test",
		muxPrefix:         normalizePrefix("/resume-operation/"),
		taskQueue:         q,
		resumableHandlers: &sync.Map{},
	}
	client.resumableHandlers.Store("create-agent", ResumeHandler(func(op *Operation) {
		resumed <- op
	}))

	op := &Operation{
		row: &OperationRow{
			Operation: &longrunningpb.Operation{Name: "operations/test-op"},
		},
		Ctx:    context.Background(),
		client: client,
	}

	if err := op.ResumeViaTasks("create-agent", 0); err != nil {
		t.Fatalf("ResumeViaTasks() error = %v, want nil with the local fallback", err)
	}
	if op.OperationPb().GetDone() {
		t.Fatal("operation was marked done by a scheduling failure")
	}
	if got := len(tasks.requests); got < 2 {
		t.Fatalf("CreateTask calls = %d, want retries before falling back", got)
	}

	select {
	case resumedOp := <-resumed:
		if resumedOp.OperationPb().GetName() != "operations/test-op" {
			t.Fatalf("resumed operation name = %q", resumedOp.OperationPb().GetName())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the in-process fallback resume")
	}
}
