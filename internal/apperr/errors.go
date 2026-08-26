// Package apperr defines the application's typed error model. Every error
// that can reach an HTTP response is expressed as an *Error so handlers can
// translate it into a consistent JSON envelope and status code without
// switching on error strings.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a stable, machine-readable error identifier. Clients should
// branch on Code, never on Message, which is meant for humans and may
// change wording over time.
type Code string

const (
	CodeValidation          Code = "VALIDATION_ERROR"
	CodeNotFound            Code = "NOT_FOUND"
	CodeConflict            Code = "CONFLICT"
	CodeUnauthorized        Code = "UNAUTHORIZED"
	CodeForbidden           Code = "FORBIDDEN"
	CodeRateLimited         Code = "RATE_LIMITED"
	CodeIdempotencyConflict Code = "IDEMPOTENCY_KEY_REUSED"
	CodeInternal            Code = "INTERNAL_ERROR"
	CodeUnavailable         Code = "SERVICE_UNAVAILABLE"
)

// Error is the canonical application error. It carries an HTTP status,
// a stable code for clients, a human message, optional field-level detail
// for validation failures, and an optional wrapped cause for logging.
type Error struct {
	Status  int               `json:"-"`
	Code    Code              `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

// WithCause attaches an underlying error for logging purposes without
// leaking internal details to the client.
func (e *Error) WithCause(cause error) *Error {
	clone := *e
	clone.cause = cause
	return &clone
}

// Cause returns the wrapped internal error, if any.
func (e *Error) Cause() error { return e.cause }

func New(status int, code Code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func NotFound(resource string) *Error {
	return New(http.StatusNotFound, CodeNotFound, fmt.Sprintf("%s not found", resource))
}

func Validation(message string, fields map[string]string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: CodeValidation, Message: message, Fields: fields}
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, CodeConflict, message)
}

func Unauthorized(message string) *Error {
	if message == "" {
		message = "authentication is required"
	}
	return New(http.StatusUnauthorized, CodeUnauthorized, message)
}

func Forbidden(message string) *Error {
	if message == "" {
		message = "you do not have permission to perform this action"
	}
	return New(http.StatusForbidden, CodeForbidden, message)
}

func RateLimited(message string) *Error {
	if message == "" {
		message = "too many requests, please slow down"
	}
	return New(http.StatusTooManyRequests, CodeRateLimited, message)
}

func Internal(cause error) *Error {
	return (&Error{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "an unexpected error occurred"}).WithCause(cause)
}

func Unavailable(message string) *Error {
	return New(http.StatusServiceUnavailable, CodeUnavailable, message)
}

// As extracts an *Error from err, if present in its chain.
func As(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
