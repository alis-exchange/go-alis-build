package mux

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"go.alis.build/alog"
	"go.alis.build/iam/v3"
	"golang.org/x/net/http2"
)

// HTTPProxy returns a handler that proxies HTTP/1.x requests to localhost:port.
//
// The proxy preserves the incoming Host header so the upstream service sees the
// original request host. If an IAM identity is present in the request context,
// HTTPProxy forwards it to the upstream service using the identity header.
func HTTPProxy(port int) Func {
	targetURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", port),
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(targetURL)
			pr.Out.Host = pr.In.Host
			applyProxyIdentity(pr.In, pr.Out)
		},
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		proxy.ServeHTTP(w, r)
		return nil
	}
}

// HTTP2Proxy returns a handler that proxies cleartext HTTP/2 requests to localhost:port.
//
// It is intended for local gRPC-style upstreams that expect h2c. The proxy
// preserves the incoming Host header and forwards any IAM identity found in the
// request context using the identity header.
func HTTP2Proxy(port int) Func {
	targetURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", port),
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(targetURL)
			pr.Out.Host = pr.In.Host
			applyProxyIdentity(pr.In, pr.Out)
		},
		// For grpc requests, force HTTP/2
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		proxy.ServeHTTP(w, r)
		return nil
	}
}

func applyProxyIdentity(in *http.Request, out *http.Request) {
	out.Header.Del("x-alis-identity")
	identity, err := iam.FromContext(in.Context())
	if err != nil {
		alog.Debugf(in.Context(), "no identity found in proxy to: %s %s", in.Method, in.URL.Path)
		return
	}
	identity.AddHeader(out)
	alog.Debugf(in.Context(), "gateway proxy forwarded identity for %s %s as %s", in.Method, in.URL.Path, identity.Email)
}
