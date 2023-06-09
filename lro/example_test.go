package lro

import (
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"context"
	"fmt"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"time"
)

func ExampleNewClient() {
	// Create a global context object.
	ctx := context.Background()

	// create client (preferably only once at a global level in the init function of your package/service)
	lroClient, _ := NewClient(ctx, "google-project", "bigtable-instance", "tableName", "")

	// create a long-running op
	op, _ := lroClient.CreateOperation(ctx, nil)

	// kick off long-running op
	go func(operationName string) {
		// do some long-running things
		time.Sleep(10 * time.Second)
		err := fmt.Errorf("some error")
		var response *anypb.Any // this can be any valid proto message

		// handle long-running results
		if err != nil {
			_ = lroClient.SetSuccessful(ctx, operationName, response, nil)
		} else {
			_ = lroClient.SetFailed(ctx, operationName, err, nil)
		}
	}(op.Name)

	// wait for a long-running op to finish
	req := &longrunningpb.WaitOperationRequest{Name: op.Name, Timeout: durationpb.New(10 * time.Second)}
	operation, _ := lroClient.WaitOperation(ctx, req)
	if operation.Done != true {
		println("operation is not done yet")
	}
}
