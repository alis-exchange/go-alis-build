package lro_test

import (
	"context"
	"net/http"
	"time"

	lro "go.alis.build/lro/v2"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type CreateAgentState struct {
	Owner     string
	PollCount int
}

func createAgentHandler(op *lro.Operation) {
	state := &CreateAgentState{}
	if err := op.DecodePrivateState(state); err != nil {
		return
	}

	meta := &structpb.Struct{}
	_, _ = lro.UnmarshalMetadata(op, meta)

	state.PollCount++
	if err := op.SavePrivateState(state); err != nil {
		return
	}
	_ = op.Complete(&emptypb.Empty{})
}

func Example_resumeViaTasks() {
	// Provision the backing Spanner table before creating the client.
	// For neuron "launchpad-v1" the table name is:
	//   ${replace(project, "-", "_")}_launchpad_v1_Operations
	// See the package docs for the Terraform snippet that provisions the required table and TTL policy.
	//
	// Launchpad creates the client once, registers resumable handlers at startup,
	// registers the HTTP callback routes on the mux, and then creates/schedules
	// operations from RPC methods.
	mux := http.NewServeMux()
	client, err := lro.New(context.Background(), lro.Config{
		Neuron:                   "launchpad-v1",
		Project:                  "example-project",
		SpannerProject:           "example-spanner-project",
		SpannerInstance:          "example-instance",
		SpannerDatabase:          "example-db",
		CloudTasksProject:        "example-project",
		CloudTasksLocation:       "europe-west1",
		CloudTasksQueue:          "launchpad-v1-operations",
		CloudTasksServiceAccount: "alis-build@example-project.iam.gserviceaccount.com",
		Host:                     "https://launchpad-backend.example.com",
	})
	if err != nil {
		return
	}
	defer client.Close()
	if err := client.AddResumableHandlers(
		lro.ResumableHandler{Path: "create-agent", Handler: createAgentHandler},
	); err != nil {
		return
	}
	client.RegisterHTTP(mux)

	ctx := context.Background()
	op, err := client.NewOperation(ctx, "operations/"+uuid.NewString(), &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"target":         structpb.NewStringValue("streams/abc"),
			"status_message": structpb.NewStringValue("Processing content..."),
		},
	})
	if err != nil {
		return
	}

	if err := op.SavePrivateState(&CreateAgentState{
		Owner:     "users/123",
		PollCount: 0,
	}); err != nil {
		return
	}

	_ = op.ResumeViaTasks("create-agent", 5*time.Second)
}
