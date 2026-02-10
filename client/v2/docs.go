// Copyright 2022 The Alis Build Platform. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package client provides a lightweight way to create authenticated gRPC client
connections to services running on Google Cloud Run.

# NewConn

NewConn creates a gRPC client connection. For secure (TLS) connections it
configures a TokenSource so that an Authorization header is injected on each
gRPC request; tokens are cached and refreshed automatically. Set insecure to
true when testing locally (no TLS, no auth).

	conn, err := client.NewConn(ctx, "your-service-abcdef.a.run.app:443", false)
	if err != nil {
		return err
	}
	defer conn.Close()

# Options

Behavior is configured via optional functional options:

  - WithRetry enables retries on temporary connection failures (e.g. TCP resets common with Cloud Run).
  - WithoutAuth disables ID token injection for secure connections when the target does not require Cloud Run / IAM auth.
  - WithDialOptions appends gRPC dial options; options passed here are applied after NewConn's defaults and take precedence.

	conn, err := client.NewConn(ctx, host, false, client.WithRetry(), client.WithDialOptions(grpc.WithBlock()))

# Inspiration

This approach was inspired by the Cloud Run sample: Send gRPC requests with authentication.
See https://cloud.google.com/run/docs/samples/cloudrun-grpc-request-auth
*/
package client // import "go.alis.build/client/v2"
