package mux

import (
	"fmt"
	"net/http"
)

type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Msg)
}

func StatusCode(err error) int {
	if e, ok := err.(*Error); ok {
		return e.Code
	} else {
		return http.StatusInternalServerError
	}
}

func NotFoundErr(msg string, args ...any) *Error {
	return &Error{http.StatusNotFound, fmt.Sprintf(msg, args...)}
}

func InternalServerErr(msg string, args ...any) *Error {
	return &Error{http.StatusInternalServerError, fmt.Sprintf(msg, args...)}
}

func BadRequestErr(msg string, args ...any) *Error {
	return &Error{http.StatusBadRequest, fmt.Sprintf(msg, args...)}
}

func UnauthorizedErr(msg string, args ...any) *Error {
	return &Error{http.StatusUnauthorized, fmt.Sprintf(msg, args...)}
}

func ForbiddenErr(msg string, args ...any) *Error {
	return &Error{http.StatusForbidden, fmt.Sprintf(msg, args...)}
}

func ConflictErr(msg string, args ...any) *Error {
	return &Error{http.StatusConflict, fmt.Sprintf(msg, args...)}
}

func TooManyRequestsErr(msg string, args ...any) *Error {
	return &Error{http.StatusTooManyRequests, fmt.Sprintf(msg, args...)}
}

func ServiceUnavailableErr(msg string, args ...any) *Error {
	return &Error{http.StatusServiceUnavailable, fmt.Sprintf(msg, args...)}
}

func UnprocessableEntityErr(msg string, args ...any) *Error {
	return &Error{http.StatusUnprocessableEntity, fmt.Sprintf(msg, args...)}
}

func NotImplementedErr(msg string, args ...any) *Error {
	return &Error{http.StatusNotImplemented, fmt.Sprintf(msg, args...)}
}

func GatewayTimeoutErr(msg string, args ...any) *Error {
	return &Error{http.StatusGatewayTimeout, fmt.Sprintf(msg, args...)}
}

func MethodNotAllowedErr(msg string, args ...any) *Error {
	return &Error{http.StatusMethodNotAllowed, fmt.Sprintf(msg, args...)}
}

func BadGatewayErr(msg string, args ...any) *Error {
	return &Error{http.StatusBadGateway, fmt.Sprintf(msg, args...)}
}

func PreconditionFailedErr(msg string, args ...any) *Error {
	return &Error{http.StatusPreconditionFailed, fmt.Sprintf(msg, args...)}
}

func PayloadTooLargeErr(msg string, args ...any) *Error {
	return &Error{http.StatusRequestEntityTooLarge, fmt.Sprintf(msg, args...)}
}
