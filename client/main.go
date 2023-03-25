package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	insecureGrpc "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/status"
	"strings"
)

type grpcTokenSource struct {
	oauth.TokenSource
}

/*
NewConn creates a new gRPC connection.
  - host should be of the form domain:port, for example: `your-app-on-cloudrun-abcdef-ew.a.run.app:443`
  - set insecure to `true` when testing your gRPC server locally.

This approach was inspired by the example provided on the following URL:
https://cloud.google.com/run/docs/samples/cloudrun-grpc-request-auth.

However, instead of manually adding the Authorization header to each gRPC request, this method implements a
TokenSource pattern. By configuring the gRPC connection once, tokens are automatically injected with each subsequent
gRPC request.

Tokens generally have a one-hour expiration time, and the TokenSource logic caches and automatically
refreshes the token upon expiration. This greatly simplifies token recycling within your service.
*/
func NewConn(ctx context.Context, host string, insecure bool) (*grpc.ClientConn, error) {

	// Validate the host argument using a regular expression to ensure it matches the required format
	// of "hostname:port".
	err := validateArgument("host", host, `^[a-zA-Z0-9.-]+:\d+$`)
	if err != nil {
		return nil, err
	}

	var opts []grpc.DialOption
	if host != "" {
		opts = append(opts, grpc.WithAuthority(host))
	}

	if insecure {
		// If the connection is insecure, add an insecure transport credentials option to the opts array.
		opts = append(opts, grpc.WithTransportCredentials(insecureGrpc.NewCredentials()))
	} else {
		// If the connection is secure, get the system root CAs and create a transport credentials option
		// using TLS with the system root CAs.
		systemRoots, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		cred := credentials.NewTLS(&tls.Config{RootCAs: systemRoots})
		opts = append(opts, grpc.WithTransportCredentials(cred))

		// use a tokenSource to automatically inject tokens with each underlying client request
		// With Cloud Run, the audience is the URL of the service you are invoking.
		audience := "https://" + strings.Split(host, ":")[0]
		tokenSource, err := idtoken.NewTokenSource(ctx, audience, option.WithAudiences(audience))
		if err != nil {
			return nil, status.Errorf(
				codes.Unauthenticated,
				"NewTokenSource: %s", err,
			)
		}
		// Add a per-RPC credentials option to the opts array using a grpcTokenSource instance created
		// with an oauth.TokenSource instance created from the tokenSource.
		opts = append(opts, grpc.WithPerRPCCredentials(grpcTokenSource{
			TokenSource: oauth.TokenSource{
				TokenSource: tokenSource,
			},
		}))
	}

	return grpc.Dial(host, opts...)
}
