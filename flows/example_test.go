package flows

import (
	"context"
	"errors"
)

func ExampleNew() {
	ctx := context.Background()

	// Create a new client -
	client, _ := NewClient("your-google-project", nil)

	// Within your RPC method, instantiate a new flow, and typically defer publishing it.
	flow, _ := client.NewFlow(ctx)
	defer flow.Publish()

	step := flow.NewStep("1.1", "Some description")

	// Failed example
	err := errors.New("some error")
	step.Failed(err)

	// State Changes
	step.Done()
	step.AwaitingInput()
	step.Queued()

	// Updating the Display name of a step
	step.WithDisplayName("A New display name of the step")
}
