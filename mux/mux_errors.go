package mux

import (
	"fmt"
	"net/http"
)

// Error represents an HTTP error that can be returned from a Func.
//
// When a handler returns an *Error, mux writes Code as the HTTP status and Msg
// as the response body. Error also implements the standard error interface for
// logging and wrapping by callers.
type Error struct {
	Code int
	Msg  string
}

// Error formats e as "<status-code>: <message>".
func (e *Error) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Msg)
}

// StatusCode returns the HTTP status code represented by err.
//
// If err is an *Error, its Code field is returned. All other error values are
// treated as internal server errors.
func StatusCode(err error) int {
	if e, ok := err.(*Error); ok {
		return e.Code
	} else {
		return http.StatusInternalServerError
	}
}

// NotFoundErr returns a 404 Not Found error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func NotFoundErr(msg string, args ...any) *Error {
	return &Error{http.StatusNotFound, fmt.Sprintf(msg, args...)}
}

// InternalServerErr returns a 500 Internal Server Error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func InternalServerErr(msg string, args ...any) *Error {
	return &Error{http.StatusInternalServerError, fmt.Sprintf(msg, args...)}
}

// BadRequestErr returns a 400 Bad Request error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func BadRequestErr(msg string, args ...any) *Error {
	return &Error{http.StatusBadRequest, fmt.Sprintf(msg, args...)}
}

// UnauthorizedErr returns a 401 Unauthorized error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func UnauthorizedErr(msg string, args ...any) *Error {
	return &Error{http.StatusUnauthorized, fmt.Sprintf(msg, args...)}
}

// ForbiddenErr returns a 403 Forbidden error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func ForbiddenErr(msg string, args ...any) *Error {
	return &Error{http.StatusForbidden, fmt.Sprintf(msg, args...)}
}

// ConflictErr returns a 409 Conflict error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func ConflictErr(msg string, args ...any) *Error {
	return &Error{http.StatusConflict, fmt.Sprintf(msg, args...)}
}

// TooManyRequestsErr returns a 429 Too Many Requests error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func TooManyRequestsErr(msg string, args ...any) *Error {
	return &Error{http.StatusTooManyRequests, fmt.Sprintf(msg, args...)}
}

// ServiceUnavailableErr returns a 503 Service Unavailable error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func ServiceUnavailableErr(msg string, args ...any) *Error {
	return &Error{http.StatusServiceUnavailable, fmt.Sprintf(msg, args...)}
}

// UnprocessableEntityErr returns a 422 Unprocessable Entity error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func UnprocessableEntityErr(msg string, args ...any) *Error {
	return &Error{http.StatusUnprocessableEntity, fmt.Sprintf(msg, args...)}
}

// NotImplementedErr returns a 501 Not Implemented error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func NotImplementedErr(msg string, args ...any) *Error {
	return &Error{http.StatusNotImplemented, fmt.Sprintf(msg, args...)}
}

// GatewayTimeoutErr returns a 504 Gateway Timeout error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func GatewayTimeoutErr(msg string, args ...any) *Error {
	return &Error{http.StatusGatewayTimeout, fmt.Sprintf(msg, args...)}
}

// MethodNotAllowedErr returns a 405 Method Not Allowed error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func MethodNotAllowedErr(msg string, args ...any) *Error {
	return &Error{http.StatusMethodNotAllowed, fmt.Sprintf(msg, args...)}
}

// BadGatewayErr returns a 502 Bad Gateway error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func BadGatewayErr(msg string, args ...any) *Error {
	return &Error{http.StatusBadGateway, fmt.Sprintf(msg, args...)}
}

// PreconditionFailedErr returns a 412 Precondition Failed error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func PreconditionFailedErr(msg string, args ...any) *Error {
	return &Error{http.StatusPreconditionFailed, fmt.Sprintf(msg, args...)}
}

// PayloadTooLargeErr returns a 413 Payload Too Large error.
//
// The message is formatted with fmt.Sprintf using msg and args.
func PayloadTooLargeErr(msg string, args ...any) *Error {
	return &Error{http.StatusRequestEntityTooLarge, fmt.Sprintf(msg, args...)}
}
