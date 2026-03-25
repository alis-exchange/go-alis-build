package lro_test

import (
	"context"
	"net/http"
	"time"

	lro "go.alis.build/lro/v2"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type CreateAgentState struct {
	Owner     string
	PollCount int
}

func Example_resumeViaTasks() {
	mux := http.NewServeMux()
	client, err := lro.New("launchpad-v1", mux, lro.WithHost("https://launchpad-backend.example.com"))
	if err != nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	op, err := client.NewOperation(ctx, "operations/example-123", &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"target":         structpb.NewStringValue("streams/abc"),
			"status_message": structpb.NewStringValue("Processing content..."),
		},
	})
	if err != nil {
		return
	}

	op.SavePrivateState(&CreateAgentState{
		Owner:     "users/123",
		PollCount: 0,
	})

	_ = op.ResumeViaTasks(func(op *lro.Operation) {
		state := &CreateAgentState{}
		op.DecodePrivateState(state)

		meta := &structpb.Struct{}
		_, _ = lro.UnmarshalMetadata(op, meta)

		state.PollCount++
		op.SavePrivateState(state)
		op.Complete(&emptypb.Empty{})
	}, 5*time.Second)
}
