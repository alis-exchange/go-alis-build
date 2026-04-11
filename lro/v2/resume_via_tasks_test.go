package lro

import (
	context "context"
	"net/http"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
)

func TestOperationResumeViaTasksWithoutHTTPHandlersUsesLocalScheduler(t *testing.T) {
	var (
		gotMux  *http.ServeMux
		gotURL  string
		gotTime time.Time
	)

	client := &Client{
		host:      "https://example.test",
		muxPrefix: normalizePrefix("/resume-operation/"),
		taskQueue: &queue{
			localScheduler: func(mux *http.ServeMux, url string, scheduleTime time.Time) error {
				gotMux = mux
				gotURL = url
				gotTime = scheduleTime
				return nil
			},
		},
		resumableHandlers: &sync.Map{},
	}
	client.resumableHandlers.Store("create-agent", ResumeHandler(func(*Operation) {}))

	op := &Operation{
		row: &OperationRow{
			Operation: &longrunningpb.Operation{Name: "operations/test-op"},
		},
		Ctx:    context.Background(),
		client: client,
	}

	before := time.Now()
	if err := op.ResumeViaTasks("create-agent", 5*time.Second); err != nil {
		t.Fatalf("ResumeViaTasks() error = %v", err)
	}

	if gotMux != nil {
		t.Fatalf("local scheduler mux = %#v, want nil when HTTP handlers are not registered", gotMux)
	}
	if gotURL != "https://example.test/resume-operation/create-agent?operation=operations/test-op" {
		t.Fatalf("local scheduler url = %q", gotURL)
	}
	if gotTime.Before(before.Add(4*time.Second)) || gotTime.After(before.Add(6*time.Second)) {
		t.Fatalf("local scheduler scheduleTime = %v, want approximately 5 seconds from now", gotTime)
	}
}
