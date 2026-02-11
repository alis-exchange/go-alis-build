package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"strings"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	insecureGrpc "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// NewConnOptions holds configuration for NewConn. It is configured via NewConnOption functions.
type NewConnOptions struct {
	retry       bool
	withoutAuth bool
	dialOpts    []grpc.DialOption
}

// NewConnOption configures optional behavior for NewConn.
type NewConnOption func(*NewConnOptions)

// WithRetry enables retries on temporary connection failures (e.g. TCP resets common with Cloud Run).
// When enabled, unary calls are retried with exponential backoff for Unavailable errors, up to 5 times.
func WithRetry() NewConnOption {
	return func(opts *NewConnOptions) {
		opts.retry = true
	}
}

// WithoutAuth configures the client to send the ID token in the 'x-serverless-authorization' header
// instead of the default 'authorization' header. Use it when you want to supply your own Authorization
// header (e.g. for application-level auth) without it being overwritten by the Cloud Run / IAM ID token.
// The receiving side must accept the token from x-serverless-authorization (e.g. proxy or backend config).
func WithoutAuth() NewConnOption {
	return func(opts *NewConnOptions) {
		opts.withoutAuth = true
	}
}

// WithDialOptions appends gRPC dial options that are applied after NewConn's defaults.
// Options passed here take precedence over defaults (e.g. authority, transport credentials, per-RPC credentials, interceptors).
func WithDialOptions(opts ...grpc.DialOption) NewConnOption {
	return func(o *NewConnOptions) {
		o.dialOpts = append(o.dialOpts, opts...)
	}
}

// customHeaderCredentials implements credentials.PerRPCCredentials.
// It injects the token into a configurable header.
type customHeaderCredentials struct {
	tokenSource oauth2.TokenSource
	header      string
}

func (c customHeaderCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	token, err := c.tokenSource.Token()
	if err != nil {
		return nil, err
	}
	// token.Type() usually returns "Bearer", but we ensure it's handled.
	tokenType := token.Type()
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return map[string]string{
		c.header: tokenType + " " + token.AccessToken,
	}, nil
}

func (c customHeaderCredentials) RequireTransportSecurity() bool {
	return true
}

// NewConn creates a new gRPC client connection.
//
// Parameters:
//   - host must be of the form "hostname:port", for example: your-app-on-cloudrun-abcdef-ew.a.run.app:443
//   - insecure: set true when testing your gRPC server locally (no TLS, no auth)
//
// Options (e.g. WithRetry, WithoutAuth, WithDialOptions) configure behavior. Any dial options passed via
// WithDialOptions are applied after NewConn's defaults, so they take precedence over default authority,
// transport credentials, per-RPC credentials, and interceptors.
//
// For secure connections, NewConn configures the connection once and then injects an Authorization header
// (or X-Serverless-Authorization if WithoutAuth is set) on each gRPC request via a TokenSource.
// Tokens have a one-hour expiration; the TokenSource caches and refreshes them automatically.
func NewConn(ctx context.Context, host string, insecure bool, opts ...NewConnOption) (*grpc.ClientConn, error) {
	// Validate the host argument using a regular expression to ensure it matches the required format
	// of "hostname:port".
	err := validateArgument("host", host, `^[a-zA-Z0-9.-]+:\d+$`)
	if err != nil {
		return nil, err
	}

	options := &NewConnOptions{
		retry:       false,
		withoutAuth: false,
		dialOpts:    []grpc.DialOption{},
	}
	for _, opt := range opts {
		opt(options)
	}

	// Build default dial options first; user options (options.dialOpts) are appended last so they take precedence.
	baseOpts := make([]grpc.DialOption, 0)

	if host != "" {
		baseOpts = append(baseOpts, grpc.WithAuthority(host))
	}

	if insecure {
		baseOpts = append(baseOpts, grpc.WithTransportCredentials(insecureGrpc.NewCredentials()))
	} else {
		systemRoots, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		cred := credentials.NewTLS(&tls.Config{RootCAs: systemRoots})
		baseOpts = append(baseOpts, grpc.WithTransportCredentials(cred))

		// Default: send token in "authorization". If WithoutAuth is set, use "x-serverless-authorization"
		// so the caller can set their own Authorization header.
		headerName := "authorization"
		if options.withoutAuth {
			headerName = "x-serverless-authorization"
		}

		audience := "https://" + strings.Split(host, ":")[0]
		tokenSource, err := idtoken.NewTokenSource(ctx, audience, option.WithAudiences(audience))
		if err != nil {
			return nil, status.Errorf(
				codes.Unauthenticated,
				"NewTokenSource: %s", err,
			)
		}
		baseOpts = append(baseOpts, grpc.WithPerRPCCredentials(customHeaderCredentials{
			tokenSource: tokenSource,
			header:      headerName,
		}))
	}

	if options.retry {
		retryOpts := []grpc_retry.CallOption{
			grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100 * time.Millisecond)),
			grpc_retry.WithCodes(codes.Unavailable),
			grpc_retry.WithMax(5),
		}
		baseOpts = append(baseOpts, grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpts...)))
	}

	// User options last so they override defaults (e.g. authority, credentials, interceptors).
	allOpts := append(baseOpts, options.dialOpts...)
	return grpc.NewClient(host, allOpts...)
}

// NewConnWithRetry is the same as NewConn with retry enabled for temporary TCP connection resets (common with Cloud Run).
//
// Deprecated: Use NewConn with WithRetry() instead.
func NewConnWithRetry(ctx context.Context, host string, insecure bool, opts ...NewConnOption) (*grpc.ClientConn, error) {
	return NewConn(ctx, host, insecure, append(opts, WithRetry())...)
}
