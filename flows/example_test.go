package flows

import (
	"context"
	"errors"
)

func ExampleNew() {
	ctx := context.Background()

	// Create a new client -
	client, _ := NewClient("your-google-project", WithTopic("flows"))

	// Within your RPC method, instantiate a new flow, and typically defer publishing it.
	flow, err := client.NewFlow(ctx)
	if err != nil {
	}
	defer flow.Publish()

	step, ctx, err := flow.NewStep("1.1", "Some title")

	// Failed example
	err = errors.New("some error")
	step.Failed(err)

	// State Changes
	step.Done()
	step.AwaitingInput()
	step.Queued()

	// Publish at a particular step
	_ = step.AwaitingInput().Publish()

	// Updating the Display name of a step
	step.WithTitle("A New title for the step")
}
