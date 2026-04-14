package edge

import (
	"fmt"
	"net/http"

	"go.alis.build/iam/v3"
)

// StripIdentityHeaders removes all trusted IAM identity headers so an edge
// service can safely overwrite them before proxying a request downstream.
func StripIdentityHeaders(headers http.Header) {
	iam.StripAuthenticatedIdentityHeaders(headers)
	iam.StripAssertedIdentityHeaders(headers)
}

// ApplyAuthenticatedIdentity overwrites the trusted authenticated transport
// identity headers on headers.
func ApplyAuthenticatedIdentity(headers http.Header, identity *iam.Identity) error {
	if headers == nil {
		return fmt.Errorf("headers are required")
	}

	iam.StripAuthenticatedIdentityHeaders(headers)
	if identity == nil {
		return nil
	}

	identityHeaders, err := identity.AuthenticatedHeaders()
	if err != nil {
		return err
	}
	copyHeaders(headers, identityHeaders)
	return nil
}

// ApplyCallerIdentity overwrites the trusted asserted caller headers on
// headers.
func ApplyCallerIdentity(headers http.Header, identity *iam.Identity) error {
	if headers == nil {
		return fmt.Errorf("headers are required")
	}

	iam.StripAssertedIdentityHeaders(headers)
	if identity == nil {
		return nil
	}

	identityHeaders, err := identity.AssertedHeaders()
	if err != nil {
		return err
	}
	copyHeaders(headers, identityHeaders)
	return nil
}

// PrepareForwardedHeaders clears any inbound trusted IAM identity headers and
// replaces them with the supplied authenticated transport principal and
// optional asserted caller.
func PrepareForwardedHeaders(headers http.Header, authenticated *iam.Identity, caller *iam.Identity) error {
	if headers == nil {
		return fmt.Errorf("headers are required")
	}

	StripIdentityHeaders(headers)
	if err := ApplyAuthenticatedIdentity(headers, authenticated); err != nil {
		return err
	}
	if err := ApplyCallerIdentity(headers, caller); err != nil {
		return err
	}
	return nil
}

func copyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
}
