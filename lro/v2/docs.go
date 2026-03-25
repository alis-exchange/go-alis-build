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
	        name     = "Method"
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
	if err := client.RegisterResumableHandler("CreateAgent", createAgentHandler); err != nil {
		return err
	}
	if err := client.RegisterHTTPHandlers(mux); err != nil {
		return err
	}

	op, err := client.NewOperation(ctx, "operations/123", metadata)

Services that use ALIS-managed infrastructure can construct the client from env:

	client, err := lro.NewFromEnv(ctx, "launchpad-v1")
	if err != nil {
		return err
	}
	if err := client.RegisterResumableHandler("CreateAgent", createAgentHandler); err != nil {
		return err
	}
	if err := client.RegisterHTTPHandlers(mux); err != nil {
		return err
	}

The host inferred from `ALIS_RUN_HASH` can be overridden when needed:

	client, err := lro.NewFromEnv(ctx, "launchpad-v1", lro.WithHost("https://launchpad-backend.example.com"))

Importing the package never validates env vars or panics.
*/
package lro // import "go.alis.build/lro/v2"
