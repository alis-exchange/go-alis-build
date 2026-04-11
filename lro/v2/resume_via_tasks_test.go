package lro

import (
	context "context"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
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
