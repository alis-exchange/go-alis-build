package lro

// func ExampleNewOperation() {
// 	// Create a global context object.
// 	ctx := context.Background()

// 	// create client (preferably only once at a global level in the init function of your package/service)
// 	client, _ := NewClient(ctx, &SpannerConfig{}, nil)

// 	// An simple illustration of using an LRO
// 	exampleRpcMethod := func(ctx context.Context, req *proto.Message) (*longrunningpb.Operation, error) {
// 		// Create an Operation object used to manage LRO activities
// 		op, err := NewOperation(ctx, client)
// 		if err != nil {
// 			return nil, err
// 		}

// 		// 2. We'll ship the business logic to a go routine to run as a background task
// 		go func() error {
// 			// Set the context for this background task, to avoid context.Cancel events from the parent ctx.
// 			ctx := context.WithoutCancel(ctx)
// 			_ = ctx

// 			// Simulating some longer running tasks...
// 			time.Sleep(45 * time.Second)

// 			var res proto.Message // replace with actual Response definition.
// 			return op.Done(res)
// 		}()

// 		// 3. Return the LRO to the caller
// 		return op.ReturnRPC()
// 	}
// 	_, _ = exampleRpcMethod(ctx, nil)
// }

// func ExampleNewResumableOperation() {
// 	// Create a global context object.
// 	ctx := context.Background()

// 	// create client (preferably only once at a global level in the init function of your package/service)
// 	client, _ := NewClient(ctx, &SpannerConfig{}, WithWorkflows(&WorkflowsConfig{}))

// 	// An illustration of using a resumable LRO
// 	exampleRpcMethod := func(ctx context.Context, req *proto.Message) (*longrunningpb.Operation, error) {
// 		// Checkpoint is a custom object used to keep track of the next step, state of variable etc.
// 		type MyCheckpoint struct {
// 			Next string
// 			Lro2 string
// 		}
// 		op, err := NewOperation(ctx, client, WithResumeState[*MyCheckpoint](&MyCheckpoint{
// 			Next: "",
// 			Lro2: "",
// 		}))
// 		if err != nil {
// 			return nil, err
// 		}

// 		// 2. We'll ship the business logic to a go routine to run as a background task
// 		go func() {
// 			// Set the context for this background task, to avoid context.Cancel events from the parent ctx.
// 			ctx := context.WithoutCancel(ctx)
// 			_ = ctx

// 			// If a checkpoint was found in the context, simply jump to the relevant point.
// 			if checkpoint != nil {
// 				switch checkpoint.Next {
// 				case "step1":
// 					goto step1
// 				}
// 			}

// 			// Perform some longrunning task which require a long (more than 10min?) waiting period
// 			{
// 				// Make a hit to a method requiring us to wait for a short period.
// 				lro1 := &longrunningpb.Operation{}
// 				client.Wait(ctx, lro1.GetName(), time.Minute*3, nil, nil)

// 				// Make a heit to a method requiring us to wait for a long period
// 				lro2 := &longrunningpb.Operation{}

// 				// We'll now make use of the resume functionality.
// 				// First, we will keep track of the name, which we'll use when we resume.
// 				checkpoint.Lro2 = lro2.GetName()

// 				// In development mode, simply wait for the relevant LRO(s) to complete.
// 				if op.DevMode() {
// 					op.WaitSync([]string{lro2.Name}, nil)
// 				} else {
// 					pollEndpoint := "https://..."
// 					err = op.WaitAsync([]string{lro2.GetName()}, checkpoint, WithPollEndpoint(pollEndpoint))
// 					if err != nil {
// 						op.Error(err)
// 						return
// 					}
// 				}
// 			}

// 		step1:
// 			{
// 				// We'll now use the value in the Checkpoint to retrieve the relevant
// 				// metadata and response values
// 				var lro2Response proto.Message = nil // replace with actual non-nil types
// 				var lro2Metadata proto.Message = nil // replace with actual non-nil types
// 				err := client.UnmarshalOperation(ctx, checkpoint.Lro2, lro2Response, lro2Metadata)
// 				if err != nil {
// 					op.Error(err)
// 					return
// 				}
// 			}

// 			var res proto.Message // replace with actual Response definition.
// 			err = op.Done(res)
// 			if err != nil {
// 				// Alert, this should never happen
// 				return
// 			}
// 		}()

// 		// 3. Return the LRO to the caller
// 		return op.Get()
// 	}
// 	_, _ = exampleRpcMethod(ctx, nil)
// }
