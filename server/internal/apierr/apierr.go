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

// Error codes from F003 (accounts and authentication).

// CSRF refuses a cookie-authenticated request that isn't from the web app.
func CSRF() *Error {
	return New(http.StatusForbidden, "csrf", "missing X-Plaid-Client header or cross-origin request")
}

// InvalidCredentials never says which of username or password was wrong.
// Status is 401 at sign-in, 403 when re-checking the password of a session.
func InvalidCredentials(status int) *Error {
	return New(status, "invalid_credentials", "wrong username or password")
}

// SetupComplete refuses first-run setup once an account exists.
func SetupComplete() *Error {
	return New(http.StatusConflict, "setup_complete", "setup is already complete")
}

// SetupCodeRequired asks a non-local client for the setup code from the log.
func SetupCodeRequired() *Error {
	return New(http.StatusForbidden, "setup_code_required", "setup from outside the local network needs the setup code from the server log")
}

// PasswordChangeRequired blocks a temporary password until it's changed.
func PasswordChangeRequired() *Error {
	return New(http.StatusForbidden, "password_change_required", "choose a new password first")
}

// WeakPassword rejects a password; the detail says why.
func WeakPassword(field, problem string) *Error {
	e := New(http.StatusUnprocessableEntity, "weak_password", "password is too short, too long or too common")
	e.Details = []Detail{{Field: field, Code: problem}}
	return e
}

// UsernameTaken means another account already has this username.
func UsernameTaken() *Error {
	return New(http.StatusConflict, "username_taken", "username already taken")
}

// LastAdmin refuses to remove the last active admin.
func LastAdmin() *Error {
	return New(http.StatusConflict, "last_admin", "at least one active admin must remain")
}

// LastProfile refuses to delete an account's only profile.
func LastProfile() *Error {
	return New(http.StatusConflict, "last_profile", "an account needs at least one profile")
}

// InvalidPIN means a profile PIN was missing or wrong.
func InvalidPIN() *Error {
	return New(http.StatusForbidden, "invalid_pin", "wrong PIN")
}

// ProfileLocked means too many wrong PINs for this profile on this session.
func ProfileLocked(retryAfter time.Duration) *Error {
	e := TooManyAttempts(retryAfter)
	e.Status, e.Code, e.Message = http.StatusLocked, "profile_locked", "too many wrong PINs, profile locked for a while"
	return e
}

// PairingNotFound covers unknown codes, wrong poll secrets and tokens
// already handed over.
func PairingNotFound() *Error {
	return New(http.StatusNotFound, "pairing_not_found", "no such pairing")
}

// PairingExpired means the code is older than 10 minutes or already used.
func PairingExpired() *Error {
	return New(http.StatusGone, "pairing_expired", "pairing code expired")
}
