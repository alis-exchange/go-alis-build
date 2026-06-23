# trace

OpenTelemetry tracing helpers for Alis services running on Google Cloud Run.

The package configures a Google Cloud Trace exporter, sets `service.name`, and
provides gRPC client/server options backed by `otelgrpc`.

```go
shutdown, err := trace.Start(ctx, trace.Config{
	ServiceName: "skills-v1",
	ProjectID:   trace.ProjectIDFromEnv(),
	SampleRatio: 0.7,
})
if err != nil {
	log.Fatal(err)
}
defer shutdown(context.Background())

grpcServer := grpc.NewServer(
	trace.GRPCServerOption(),
	grpc.UnaryInterceptor(unaryInterceptor),
)
```

For `go.alis.build/client/v2`:

```go
conn, err := client.NewConn(ctx, host, false,
	client.WithDialOptions(trace.GRPCDialOption()),
)
```
