// Copyright 2022 The Alis Build Platform. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package client provides a lightweight way to create authenticated gRPC client
connections to services running on Google Cloud Run.

# NewConn

NewConn creates a gRPC client connection. For secure (TLS) connections it
injects a Cloud Run ID token on each gRPC request via a TokenSource (default
header: authorization). Use WithoutAuth() to send the token in
x-serverless-authorization instead so the caller's Authorization header is
preserved; the receiving side must accept the token from that header. Tokens
are cached and refreshed automatically. Set insecure to true when testing
locally (no TLS, no auth).

	conn, err := client.NewConn(ctx, "your-service-abcdef.a.run.app:443", false)
	if err != nil {
		return err
	}
	defer conn.Close()

# Options

Behavior is configured via optional functional options:

  - WithRetry enables retries on temporary connection failures (e.g. TCP resets common with Cloud Run).

  - WithoutAuth sends the ID token in x-serverless-authorization instead of authorization so the caller can set their own Authorization header.

  - WithDialOptions appends gRPC dial options; options passed here are applied after NewConn's defaults and take precedence.

    conn, err := client.NewConn(ctx, host, false, client.WithRetry(), client.WithDialOptions(grpc.WithBlock()))

# Inspiration

This approach was inspired by the Cloud Run sample: Send gRPC requests with authentication.
See https://cloud.google.com/run/docs/samples/cloudrun-grpc-request-auth
*/
package client // import "go.alis.build/client/v2"
