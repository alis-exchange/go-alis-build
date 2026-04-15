// Package mux provides a simple http.ServeMux wrapper.
package mux

import (
	"errors"
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var (
	mux     = http.NewServeMux()
	gateway Middleware
)

type (
	Func       func(w http.ResponseWriter, r *http.Request) error
	Middleware func(w http.ResponseWriter, r *http.Request, handler Func) error
)

func AddGateway(gw Middleware) {
	gateway = gw
}

func Handle(pattern string, handleFunc Func, middlewares ...Middleware) {
	mux.HandleFunc(pattern, handler(handleFunc, middlewares...))
}

func Options(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("OPTIONS "+pattern, handleFunc, middlewares...)
}

func Get(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("GET "+pattern, handleFunc, middlewares...)
}

func Post(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("POST "+pattern, handleFunc, middlewares...)
}

func Patch(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("PATCH "+pattern, handleFunc, middlewares...)
}

func Put(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("PUT "+pattern, handleFunc, middlewares...)
}

func Delete(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("DELETE "+pattern, handleFunc, middlewares...)
}

func handler(handler Func, middlewares ...Middleware) func(w http.ResponseWriter, r *http.Request) {
	var mws []Middleware
	mws = append(mws, middlewares...)

	for i := len(mws) - 1; i >= 0; i-- {
		mw := mws[i]
		next := handler
		handler = func(w http.ResponseWriter, r *http.Request) error {
			return mw(w, r, next)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		if gateway != nil {
			err = gateway(w, r, handler)
		} else {
			err = handler(w, r)
		}
		if err != nil {
			if httpErr, ok := errors.AsType[*Error](err); ok {
				http.Error(w, httpErr.Msg, httpErr.Code)
			} else {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	}
}

func ListenAndServe(addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}
	return server.ListenAndServe()
}
