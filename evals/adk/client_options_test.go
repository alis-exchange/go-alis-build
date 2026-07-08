package adk

import (
	"testing"
	"time"
)

func TestNewHTTPClient_defaultTimeout(t *testing.T) {
	t.Parallel()

	c := NewHTTPClient("http://example.com")
	if got := c.httpClient.Timeout; got != defaultTimeout {
		t.Fatalf("default timeout = %v, want %v", got, defaultTimeout)
	}
}

func TestNewHTTPClient_withTimeoutPositive(t *testing.T) {
	t.Parallel()

	want := 5 * time.Second
	c := NewHTTPClient("http://example.com", WithTimeout(want))
	if got := c.httpClient.Timeout; got != want {
		t.Fatalf("timeout = %v, want %v", got, want)
	}
}

func TestNewHTTPClient_withTimeoutZeroDisables(t *testing.T) {
	t.Parallel()

	c := NewHTTPClient("http://example.com", WithTimeout(0))
	if got := c.httpClient.Timeout; got != 0 {
		t.Fatalf("timeout with WithTimeout(0) = %v, want 0 (disabled)", got)
	}
}

func TestNewHTTPClient_withTimeoutNegativeDisables(t *testing.T) {
	t.Parallel()

	c := NewHTTPClient("http://example.com", WithTimeout(-1*time.Second))
	if got := c.httpClient.Timeout; got != 0 {
		t.Fatalf("timeout with negative WithTimeout = %v, want 0 (disabled)", got)
	}
}

func TestNewHTTPClient_defaultPathPrefix(t *testing.T) {
	t.Parallel()

	c := NewHTTPClient("http://example.com")
	if got := c.pathPrefix; got != defaultPathPrefix {
		t.Fatalf("default pathPrefix = %q, want %q", got, defaultPathPrefix)
	}
}

func TestNewHTTPClient_withEmptyPathPrefixRestoresDefault(t *testing.T) {
	t.Parallel()

	c := NewHTTPClient("http://example.com", WithPathPrefix(""))
	if got := c.pathPrefix; got != defaultPathPrefix {
		t.Fatalf("pathPrefix with WithPathPrefix(\"\") = %q, want %q", got, defaultPathPrefix)
	}
}
