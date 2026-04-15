package iam

import (
	"errors"
	"fmt"
	"net/http"
)

// AddHeader adds the identity to the given http request's headers.
func (i *Identity) AddHeader(r *http.Request) {
	if i == nil {
		return
	}
	value := string(i.Marshal())
	r.Header.Set(string(identityCtxKey), value)
}

// Middleware provides an http handler that extracts the caller identity from
// the request headers and injects it into the request context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := MustFromHeader(r)
		ctx := identity.Context(r.Context())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromHeader returns the Identity inside the given http request's headers, if any.
func FromHeader(r *http.Request) (*Identity, error) {
	value := r.Header.Get(string(identityCtxKey))
	if value == "" {
		return nil, errors.New("no identity value found in headers")
	}
	identity, err := Unmarshal([]byte(value))
	if err != nil {
		return nil, fmt.Errorf("unmarshalling request header: %v", err)
	}
	identity.checkIfSystem()
	return identity, nil
}

// MustFromHeader does the same as FromHeader, but panics on an error.
func MustFromHeader(r *http.Request) *Identity {
	identity, err := FromHeader(r)
	if err != nil {
		panic(fmt.Sprintf("identity.MustFromHeader: %v", err))
	}
	return identity
}
