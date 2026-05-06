package iam

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	testIdentity.AddHeader(req)

	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := MustFromContext(r.Context())
		expect(t, identity.Type, testIdentity.Type)
		expect(t, identity.ID, testIdentity.ID)
		expect(t, identity.Email, testIdentity.Email)
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	expect(t, rec.Code, http.StatusNoContent)
}

func TestMiddlewareUnauthenticated(t *testing.T) {
	handlerCalled := false
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	expect(t, rec.Code, http.StatusUnauthorized)
	if handlerCalled {
		t.Fatal("handler was called")
	}
}
