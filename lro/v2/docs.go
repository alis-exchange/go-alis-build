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
  - `Method` `STRING` nullable
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
