// Package trace configures OpenTelemetry tracing for Alis services running on
// Google Cloud Run.
//
// The package exists to keep Google Cloud Trace setup consistent across Alis
// Go services. A service should not need to know the details of the Cloud Trace
// exporter, OpenTelemetry resource attributes, sampling, or gRPC stats handlers
// just to emit useful traces. This package centralizes those choices while
// still keeping tracing explicit at application startup.
//
// Start tracing from main before constructing gRPC servers or clients:
//
//	ctx := context.Background()
//	shutdown, err := trace.Start(ctx, trace.Config{
//		ServiceName: "skills-v1",
//		ProjectID:   trace.ProjectIDFromEnv(),
//		SampleRatio: 0.7,
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer shutdown(context.Background())
//
// Then attach the server option when creating a gRPC server:
//
//	grpcServer := grpc.NewServer(
//		trace.GRPCServerOption(),
//		grpc.UnaryInterceptor(unaryInterceptor),
//	)
//
// Attach the dial option to outbound clients. For go.alis.build/client/v2, pass
// it through client.WithDialOptions:
//
//	conn, err := client.NewConn(ctx, host, false,
//		client.WithDialOptions(trace.GRPCDialOption()),
//	)
//	if err != nil {
//		return err
//	}
//
// Start returns a shutdown function because the OpenTelemetry SDK batches spans
// in the background. Call the shutdown function during process shutdown so
// buffered spans are flushed to Cloud Trace.
//
// The package intentionally does not configure tracing in init. Explicit setup
// keeps tests predictable, lets applications choose their service name and
// sampling ratio, and avoids surprising Cloud Trace exporter initialization when
// a library is imported.
//
// By default, spans are exported directly to Google Cloud Trace using
// Application Default Credentials. On Cloud Run this means the service account
// attached to the revision is used. ProjectIDFromEnv reads common Cloud Run and
// Google client environment variables, and Config.ProjectID can be set directly
// when the destination project should be explicit.
//
// Config.ServiceName is required because Cloud Trace and other OpenTelemetry
// tools use the service.name resource attribute to group spans by service. Use
// the stable neuron or service id, for example "skills-v1".
//
// Sampling uses ParentBased TraceIDRatioBased sampling. ParentBased preserves an
// upstream sampling decision when the service receives a request that already
// belongs to a trace. TraceIDRatioBased controls the sampling rate for new root
// traces started by this process.
//
// gRPC instrumentation is implemented with otelgrpc stats handlers rather than
// interceptors. Stats handlers observe client and server RPC lifecycle events
// at the transport layer and are the standard OpenTelemetry instrumentation path
// for google.golang.org/grpc. GRPCServerOption instruments inbound RPCs, while
// GRPCDialOption instruments outbound RPCs and propagates the current trace
// context.
//
// The default propagator is W3C Trace Context plus Baggage, which is the normal
// OpenTelemetry propagation format and works across HTTP, gRPC, and non-Go
// services that support OpenTelemetry. Applications can provide Config.Propagator
// if they need custom propagation behavior.
package trace
