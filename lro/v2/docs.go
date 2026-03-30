/*
Package lro provides a client for managing long-running operations stored in Spanner
and resumed via Cloud Tasks.

Before using this package, provision the backing Spanner table for the target
`neuron`. The client expects a table named:

	${replace(project, "-", "_")}_${replace(neuron, "-", "_")}_Operations

The required schema is:

  - `key` `STRING` primary key, computed/stored from `Operation.name`
  - `Operation` `PROTO<google.longrunning.Operation>` required
  - `State` `BYTES` nullable
  - `ResumePoint` `STRING` nullable
  - `UpdateTime` `TIMESTAMP` required

Example Terraform:

```hcl

	resource "alis_google_spanner_table" "operations" {
	  project         = var.ALIS_MANAGED_SPANNER_PROJECT
	  instance        = var.ALIS_MANAGED_SPANNER_INSTANCE
	  database        = var.ALIS_MANAGED_SPANNER_DB
	  name            = "${replace(var.ALIS_OS_PROJECT, "-", "_")}_${replace("launchpad-v1", "-", "_")}_Operations"
	  prevent_destroy = true
	  schema = {
	    columns = [
	      {
	        name            = "key"
	        is_computed     = true
	        computation_ddl = "Operation.name"
	        is_stored       = true
	        type            = "STRING"
	        is_primary_key  = true
	        required        = true
	        unique          = true
	      },
	      {
	        name          = "Operation"
	        type          = "PROTO"
	        proto_package = "google.longrunning.Operation"
	        required      = true
	      },
	      {
	        name     = "State"
	        type     = "BYTES"
	        required = false
	      },
	      {
	        name     = "ResumePoint"
	        type     = "STRING"
	        required = false
	      },
	      {
	        name     = "UpdateTime"
	        type     = "TIMESTAMP"
	        required = true
	      },
	    ]
	  }
	}

	resource "alis_google_spanner_table_ttl_policy" "operations" {
	  project  = alis_google_spanner_table.operations.project
	  instance = alis_google_spanner_table.operations.instance
	  database = alis_google_spanner_table.operations.database
	  table    = alis_google_spanner_table.operations.name
	  column   = "UpdateTime"
	  ttl      = 90
	}

```

The v2 API uses explicit client configuration and explicit HTTP binding:

	client, err := lro.New(ctx, lro.Config{
		Neuron:                   "launchpad-v1",
		Project:                  "my-project",
		SpannerProject:           "my-spanner-project",
		SpannerInstance:          "my-spanner-instance",
		SpannerDatabase:          "my-spanner-db",
		CloudTasksProject:        "my-project",
		CloudTasksLocation:       "europe-west1",
		CloudTasksQueue:          "launchpad-v1-operations",
		CloudTasksServiceAccount: "alis-build@my-project.iam.gserviceaccount.com",
		Host:                     "https://launchpad-backend.example.com",
	})
	if err != nil {
		return err
	}
	if err := client.AddResumableHandler("create-agent", createAgentHandler); err != nil {
		return err
	}
	if err := client.RegisterHTTPHandlers(mux); err != nil {
		return err
	}
	longrunningpb.RegisterOperationsServer(grpcServer, client.OperationsServer())

	op, err := client.NewOperation(ctx, "operations/123", metadata)
	if err := op.ResumeViaTasks("create-agent", 0); err != nil {
		return err
	}

Services that use ALIS-managed infrastructure can construct the client from env:

	client, err := lro.NewFromEnv(ctx, "launchpad-v1")
	if err != nil {
		return err
	}
	if err := client.AddResumableHandler("create-agent", createAgentHandler); err != nil { return err }
	if err := client.RegisterHTTPHandlers(mux); err != nil {
		return err
	}
	longrunningpb.RegisterOperationsServer(grpcServer, client.OperationsServer())

`NewFromEnv` infers the callback host from the Cloud Run URL pattern and these
env vars:

  - `ALIS_PROJECT_NR`
  - `ALIS_REGION`

The inferred host has this form:

	https://{service}-{project-number}.{region}.run.app

That host can be overridden when needed:

	client, err := lro.NewFromEnv(ctx, "launchpad-v1", lro.WithHost("https://launchpad-backend.example.com"))

Importing the package never validates env vars or panics.

Here is a typical implementation flow:

 1. Create a shared client once during process startup:

    client, err := lro.NewFromEnv(ctx, "launchpad-v1")
    if err != nil {
    return err
    }

 2. Register resumable handlers during startup, not when the RPC is called.
    Use single or batch flows depending on the service setup:

    Register a single handler, like the `PublishAgent` flow:

    if err := client.AddResumableHandler("publish-agent", publishAgentHandler); err != nil {
    return err
    }

    Register multiple handlers in one place:

    if err := client.AddResumableHandlers(
    lro.ResumableHandler{Path: "publish-agent", Handler: publishAgentHandler},
    lro.ResumableHandler{Path: "submit-agent-feedback", Handler: submitAgentFeedbackHandler},
    lro.ResumableHandler{Path: "create-agent-from-content", Handler: createAgentFromContentHandler},
    ); err != nil {
    return err
    }

 3. Expose both the Operations API and the resumable HTTP callback routes:

    Typically, in the server.go add the following:

    longrunningpb.RegisterOperationsServer(grpcServer, client.OperationsServer())
    if err := client.RegisterHTTPHandlers(mux); err != nil {
    return err
    }

 4. In the RPC method, create the operation, attach metadata for clients, save
    private state for the resumable workflow, and schedule the first callback:

    op, err := client.NewOperation(ctx, "operations/"+uuid.NewString(), metadata)
    if err != nil {
    return nil, err
    }
    if err := op.SavePrivateState(&MyState{
    UserID: userID,
    Stream: streamName,
    }); err != nil {
    return nil, err
    }
    if err := op.ResumeViaTasks("create-agent-from-content", 0); err != nil {
    return nil, err
    }
    return op.OperationPb(), nil

 5. In the resumable handler, restore private state, optionally unmarshal and
    update metadata, then either requeue or finish the operation:

    func createAgentFromContentHandler(op *lro.Operation) {
    state := &MyState{}
    if err := op.DecodePrivateState(state); err != nil {
    op.Fail("decode private state: %v", err)
    return
    }

    meta := &pb.CreateAgentFromContentMetadata{}
    if _, err := lro.UnmarshalMetadata(op, meta); err != nil {
    op.Fail("unmarshal metadata: %v", err)
    return
    }

    meta.StatusMessage = "Waiting for content processing..."
    if err := op.SaveMetadata(meta); err != nil {
    op.Fail("save metadata: %v", err)
    return
    }

    if stillWaiting {
    if err := op.ResumeViaTasks("create-agent-from-content", 2*time.Second); err != nil {
    op.Fail("reschedule task: %v", err)
    }
    return
    }

    _ = op.Complete(response)
    }

This split is intentional:

  - operation metadata is the client-visible status surface
  - private state is for workflow-only data such as user ids, upstream resource
    names, poll counts, or serialized requests

For Cloud Tasks driven handlers that call other services, Launchpad also uses
`context.WithoutCancel(ctx)` before creating outbound RPC metadata. That avoids
propagating the Cloud Tasks dispatch deadline to downstream services that may
schedule their own async work.

Mental model for building an RPC or method that returns an LRO:

 1. At service startup, add a resumable handler for that workflow and register the
    HTTP handlers on your mux.
 2. In the RPC that creates the operation, create the LRO, persist any private
    state needed to continue later, and call `ResumeViaTasks(path, delay)`.
 3. In the resumable handler, reload state and metadata from the operation,
    advance the workflow, and either:
    - call `ResumeViaTasks(path, nextDelay)` again to continue later, or
    - call `Complete(...)` / `Fail(...)` to finish the operation.

The important design rule is that the resumable handler must be registered at
startup. Do not rely on scheduling time to create HTTP routes, because a future
Cloud Tasks callback may land on a fresh instance that never executed the
original scheduling request.
*/
package lro // import "go.alis.build/lro/v2"
