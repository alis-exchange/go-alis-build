package lro

import (
	"context"
	"fmt"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"time"
)

func ExampleNewClient() {
	// create client (preferably only once at a global level in the init function of your package/service)
	lroClient := NewClient(context.Background(), "google-project", "bigtable-instance", "tableName")

	// create a long-running op
	op, _ := lroClient.CreateOperation(context.Background(), CreateOpts{})

	// kick off long-running op
	go func(operationName string) {
		// do some long-running things
		time.Sleep(10 * time.Second)
		err := fmt.Errorf("some error")
		var response *anypb.Any //this can be any valid proto message

		// handle long-running results
		if err != nil {
			_ = lroClient.SetSuccessful(context.Background(), operationName, response, MetaOptions{})
		} else {
			_ = lroClient.SetFailed(context.Background(), operationName, &status.Status{Message: err.Error(), Code: 500}, MetaOptions{})
		}
	}(op.Name)
}
