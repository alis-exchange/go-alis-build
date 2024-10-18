package lro

import (
	"context"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func ExampleNewOperation() {
	// Create a global context object.
	ctx := context.Background()

	// create client (preferably only once at a global level in the init function of your package/service)
	client, _ := NewClient(ctx, &SpannerConfig{}, WithWorkflows(""))

	// An simple illustration of using an LRO
	exampleRpcMethod := func(ctx context.Context, req *proto.Message) (*longrunningpb.Operation, error) {
		// Create an Operation object used to manage LRO activities
		type MyState struct{}
		op, err := NewOperation[MyState](ctx, client)
		if err != nil {
			return nil, err
		}

		// 2. We'll ship the business logic to a go routine to run as a background task
		go func() error {
			// If relevant get the ResumePoint
			switch op.ResumePoint() {
			case "resumePoint1":
				goto point1
			}

			{
				// This is where we will wait...
				// Scenario 1: simply wait for 30 seconds.
				op.Wait(WithSleep(30 * time.Second))

				// Scenario 2: Make one or more hits to methods returning LROs, and wait for these to complete.
				op.Wait(WithChildOperations([]string{"operations/123", "operations/456"}))

				// Scenario 3: Customise the poll frequency and timeout
				op.Wait(WithChildOperations([]string{"operations/123", "operations/456"}), WithTimeout(10*time.Minute), WithPollFrequency(30*time.Second))

				// Scenario 4: If the underlying operations are from another product, you would need to use a different client to poll
				// the relevant GetOperation methods.  The service you connect to needs to satisfy the LRO interface as defined by google.longrunning package
				var conn grpc.ClientConnInterface // create a connection to the relevant gRPC server
				myLroClient := longrunningpb.NewOperationsClient(conn)
				op.Wait(WithChildOperations([]string{"operations/123", "operations/456"}), WithService(myLroClient))

				// Scenario 5: If, however the operations from another product does not implement the google.longrunning service, you could use
				// any service that implements a GetOperation() method, therefore satisfying the OperationsService interface defined in this package.
				var myProductClient OperationsService
				op.Wait(WithChildOperations([]string{"operations/123", "operations/456"}), WithService(myProductClient))

				// Scenario 6: Wait asynchronously for a longer time.
				op.SetState(&MyState{}) // Explicitly set the state before waiting asynchronously
				op.Wait(WithSleep(24*time.Hour), WithAsync("resumePoint1"))
				// since we explicitly configure the Async option, we need to exit this method now.
				return nil
			}

		point1:
			// Once resumed, get the last available State and do some cool stuff.
			state := op.State()
			_ = state

			// And finally mark the LRO as complete.
			var res proto.Message // replace with actual Response definition.
			return op.Done(res)
		}()

		// 3. Return the LRO to the caller
		return op.ReturnRPC()
	}
	_, _ = exampleRpcMethod(ctx, nil)
}
