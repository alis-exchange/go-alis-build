# LRO v2

`go.alis.build/lro/v2` provides a long-running operations client backed by
Google Cloud Spanner and resumed through Cloud Tasks.

## What this package does

- Persists `google.longrunning.Operation` records in Spanner.
- Exposes a standard `google.longrunning.Operations` gRPC server.
- Registers HTTP callback routes for resumable handlers.
- Stores private resumable workflow state alongside operation metadata.
- Requeues unfinished work through Cloud Tasks.

## Installation

```bash
go get go.alis.build/lro/v2
```

## Infrastructure contract

Provision the backing Spanner resources before creating the client. For a
service following the ALIS module layout, keep the LRO resources in
`infra/modules/alis.lro.v2` and wire that module from `infra/main.tf`.

The client expects:

- A Spanner table named `${replace(project, "-", "_")}_${replace(neuron, "-", "_")}_Operations`
- A Cloud Tasks queue named `{neuron}-operations`
- A callback host that matches either the explicit `Host` config or the default
  Cloud Run URL inferred by `NewFromEnv`

The full Terraform example and schema live in
[`docs.go`](./docs.go).

## Startup flow

Create one shared client at startup, register resumable handlers once, mount the
HTTP callback routes, and register the Operations API:

```go
mux := http.NewServeMux()

client, err := lro.NewFromEnv(ctx, "launchpad-v1")
if err != nil {
	return err
}
defer client.Close()

if err := client.AddResumableHandlers(
	lro.ResumableHandler{Path: "create-agent", Handler: createAgentHandler},
); err != nil {
	return err
}
client.RegisterHTTPHandlers(mux)

longrunningpb.RegisterOperationsServer(grpcServer, client.OperationsServer())
```

In an RPC, create the operation, save any private state you need for resumes,
and schedule the first callback:

```go
op, err := client.NewOperation(ctx, "operations/"+uuid.NewString(), metadata)
if err != nil {
	return nil, err
}
if err := op.SavePrivateState(&CreateAgentState{Owner: "users/123"}); err != nil {
	return nil, err
}
if err := op.ResumeViaTasks("create-agent", 0); err != nil {
	return nil, err
}
return op.OperationPb(), nil
```

See [`example_test.go`](./example_test.go) and
[`docs.go`](./docs.go) for the end-to-end flow.
