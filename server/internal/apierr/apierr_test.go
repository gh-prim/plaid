package apierr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type body struct {
	Error struct {
		Code    string   `json:"code"`
		Message string   `json:"message"`
		Details []Detail `json:"details"`
	} `json:"error"`
}

func write(t *testing.T, e *Error) (*httptest.ResponseRecorder, body) {
	t.Helper()
	rec := httptest.NewRecorder()
	Write(rec, e)
	var b body
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode %q: %v", rec.Body, err)
	}
	return rec, b
}

func TestWriteSharedShape(t *testing.T) {
	rec, b := write(t, Forbidden())
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if b.Error.Code != "forbidden" || b.Error.Message == "" {
		t.Errorf("body = %+v", b)
	}
	if b.Error.Details != nil {
		t.Errorf("details = %v, want omitted", b.Error.Details)
	}
}

func TestValidationDetails(t *testing.T) {
	rec, b := write(t, ValidationFailed(Detail{Field: "username", Code: "too_short"}))
	if rec.Code != http.StatusUnprocessableEntity || b.Error.Code != "validation_failed" {
		t.Fatalf("status %d body %+v", rec.Code, b)
	}
	if len(b.Error.Details) != 1 || b.Error.Details[0] != (Detail{"username", "too_short"}) {
		t.Errorf("details = %+v", b.Error.Details)
	}
}

func TestTooManyAttemptsSetsRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		wait time.Duration
		want string
	}{
		{90 * time.Second, "90"},
		{1500 * time.Millisecond, "2"},
		{100 * time.Millisecond, "1"},
	} {
		rec, b := write(t, TooManyAttempts(tc.wait))
		if rec.Code != http.StatusTooManyRequests || b.Error.Code != "too_many_attempts" {
			t.Fatalf("status %d body %+v", rec.Code, b)
		}
		if got := rec.Header().Get("Retry-After"); got != tc.want {
			t.Errorf("wait %v: Retry-After = %q, want %q", tc.wait, got, tc.want)
		}
	}
}

func TestWithHeaderDoesNotMutateOriginal(t *testing.T) {
	base := Unauthorized()
	_ = base.WithHeader("X-Test", "1")
	if base.Header != nil {
		t.Errorf("original header mutated: %v", base.Header)
	}
}

func TestPlaybackErrors(t *testing.T) {
	for _, c := range []struct {
		e      *Error
		status int
		code   string
	}{
		{NotPlayable("x"), http.StatusUnprocessableEntity, "not_playable"},
		{FileUnavailable(), http.StatusGone, "file_unavailable"},
		{FileNotAnalyzed(), http.StatusConflict, "file_not_analyzed"},
		{ProfileRequired(), http.StatusForbidden, "profile_required"},
		{FFmpegMissing(), http.StatusServiceUnavailable, "ffmpeg_missing"},
		{PlaybackSessionNotFound(), http.StatusNotFound, "playback_session_not_found"},
	} {
		rec, b := write(t, c.e)
		if rec.Code != c.status || b.Error.Code != c.code {
			t.Errorf("%s: %d %s", c.code, rec.Code, b.Error.Code)
		}
	}
}
