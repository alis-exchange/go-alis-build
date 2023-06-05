package lro

import (
	"context"
	"fmt"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"time"
)

func ExampleNewClient() {
	// Create a global context object.
	ctx := context.Background()

	// create client (preferably only once at a global level in the init function of your package/service)
	lroClient := NewClient(ctx, "google-project", "bigtable-instance", "tableName", "")

	// create a long-running op
	op, _ := lroClient.CreateOperation(ctx, CreateOpts{})

	// kick off long-running op
	go func(operationName string) {
		// do some long-running things
		time.Sleep(10 * time.Second)
		err := fmt.Errorf("some error")
		var response *anypb.Any // this can be any valid proto message

		// handle long-running results
		if err != nil {
			_ = lroClient.SetSuccessful(ctx, operationName, response, MetaOptions{})
		} else {
			_ = lroClient.SetFailed(ctx, operationName, &status.Status{Message: err.Error(), Code: 500}, MetaOptions{})
		}
	}(op.Name)
}
