package lro

// import (
// 	"context"
// 	"fmt"
// 	"time"

// 	"cloud.google.com/go/longrunning/autogen/longrunningpb"
// 	"google.golang.org/protobuf/types/known/anypb"
// 	"google.golang.org/protobuf/types/known/durationpb"
// )

// func ExampleNewClient() {
// 	// Create a global context object.
// 	ctx := context.Background()

// 	// create client (preferably only once at a global level in the init function of your package/service)
// 	lroClient, _ := NewClient(ctx, "google-project", "bigtable-instance", "tableName", "")

// 	// create a long-running op
// 	op, _ := lroClient.CreateOperation(ctx, nil)

// 	// kick off long-running op
// 	go func(operationName string) {
// 		// do some long-running things
// 		time.Sleep(10 * time.Second)
// 		err := fmt.Errorf("some error")
// 		var response *anypb.Any // this can be any valid proto message

// 		// handle long-running results
// 		if err != nil {
// 			op, _ = lroClient.SetSuccessful(ctx, operationName, response, nil)
// 		} else {
// 			op, _ = lroClient.SetFailed(ctx, operationName, err, nil)
// 		}
// 	}(op.Name)

// 	// wait for a long-running op to finish with a callback that prints incoming metadata
// 	req := &longrunningpb.WaitOperationRequest{Name: op.Name, Timeout: durationpb.New(10 * time.Second)}
// 	metadataHandler := func(metadata *anypb.Any) {
// 		// Assuming the metadata is a protobuf message, you can unmarshal it into a specific type
// 		// Replace `YourMetadataMessageType` with the actual type of your metadata message.
// 		//var metadataMsg {YourMetadataMessageType}
// 		//if err := anypb.UnmarshalTo(metadata, metadataMsg, nil); err != nil {
// 		//	// Handle unmarshaling error
// 		//	log.Println("Failed to unmarshal metadata:", err)
// 		//	return
// 		//}
// 		//
// 		//// Process the metadata as needed
// 		//log.Println("Received metadata:", metadataMsg)
// 		return
// 	}
// 	operation, _ := lroClient.WaitOperation(ctx, req, metadataHandler)
// 	if operation.Done != true {
// 		println("operation is not done yet")
// 	}
// }
