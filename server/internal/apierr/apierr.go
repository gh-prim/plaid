// Package apierr defines the errors the API returns to clients.
//
// Handlers return an *Error as a plain error; the HTTP layer writes it with
// the shared shape {"error": {"code", "message", "details"?}}. Any other
// error is a bug and becomes a 500.
package apierr

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Detail points at one invalid field of a request body.
type Detail struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// Error is an API error: an HTTP status, a stable code clients translate,
// and an English message for developers.
type Error struct {
	Status  int
	Code    string
	Message string
	Details []Detail
	Header  http.Header
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// New returns an error with the given status, code and message.
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// WithHeader returns a copy of e that also sets the response header key.
func (e *Error) WithHeader(key, value string) *Error {
	c := *e
	c.Header = e.Header.Clone()
	if c.Header == nil {
		c.Header = http.Header{}
	}
	c.Header.Set(key, value)
	return &c
}

// Write writes e as the shared JSON error shape.
func Write(w http.ResponseWriter, e *Error) {
	for k, v := range e.Header {
		w.Header()[k] = v
	}
	body := struct {
		Code    string   `json:"code"`
		Message string   `json:"message"`
		Details []Detail `json:"details,omitempty"`
	}{e.Code, e.Message, e.Details}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": body})
}

// BadRequest is a request the server can't decode.
func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, "bad_request", message)
}

// Unauthorized means no valid session came with the request.
func Unauthorized() *Error {
	return New(http.StatusUnauthorized, "unauthorized", "a valid session is required")
}

// Forbidden means the session is valid but not allowed to do this.
func Forbidden() *Error {
	return New(http.StatusForbidden, "forbidden", "not allowed for this account")
}

// NotFound means the resource doesn't exist (or isn't visible to the caller).
func NotFound(message string) *Error {
	return New(http.StatusNotFound, "not_found", message)
}

// ValidationFailed lists the invalid fields of a request body.
func ValidationFailed(details ...Detail) *Error {
	e := New(http.StatusUnprocessableEntity, "validation_failed", "some fields are invalid")
	e.Details = details
	return e
}

// TooManyAttempts asks the client to wait before trying again.
func TooManyAttempts(retryAfter time.Duration) *Error {
	secs := int(retryAfter.Round(time.Second) / time.Second)
	if secs < 1 {
		secs = 1
	}
	return New(http.StatusTooManyRequests, "too_many_attempts", "too many attempts, try again later").
		WithHeader("Retry-After", strconv.Itoa(secs))
}

// Internal is the response for unexpected errors. Details stay in the logs.
func Internal() *Error {
	return New(http.StatusInternalServerError, "internal_error", "internal server error")
}
