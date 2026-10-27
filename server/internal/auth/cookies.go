package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/gh-prim/plaid/server/internal/clientinfo"
)

// cookieMaxAge is the browser-side lifetime of the session cookie. Browsers
// cap cookies at 400 days; the server's sliding 90-day expiry is what counts.
const cookieMaxAge = 400 * 24 * time.Hour

// Handlers return typed responses and never see the ResponseWriter, so they
// queue cookies in the request context; jarWriter writes them before the
// response headers go out.
type (
	jarKey    struct{}
	cookieJar struct{ cookies []*http.Cookie }
)

type jarWriter struct {
	http.ResponseWriter
	jar     *cookieJar
	flushed bool
}

func (w *jarWriter) writeCookies() {
	if w.flushed {
		return
	}
	w.flushed = true
	for _, c := range w.jar.cookies {
		http.SetCookie(w.ResponseWriter, c)
	}
}

func (w *jarWriter) WriteHeader(status int) {
	w.writeCookies()
	w.ResponseWriter.WriteHeader(status)
}

func (w *jarWriter) Write(b []byte) (int, error) {
	w.writeCookies()
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the real writer.
func (w *jarWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func setCookie(ctx context.Context, c *http.Cookie) {
	if jar, ok := ctx.Value(jarKey{}).(*cookieJar); ok {
		jar.cookies = append(jar.cookies, c)
	}
}

// SetSessionCookie queues the web app's session cookie (F003-R9).
func SetSessionCookie(ctx context.Context, token string) {
	setCookie(ctx, sessionCookie(ctx, token, int(cookieMaxAge/time.Second)))
}

// ClearSessionCookie queues the removal of the session cookie.
func ClearSessionCookie(ctx context.Context) {
	setCookie(ctx, sessionCookie(ctx, "", -1))
}

func sessionCookie(ctx context.Context, value string, maxAge int) *http.Cookie {
	// Secure follows the client's scheme: PLAID is often reached over plain
	// HTTP on a LAN, where a Secure cookie would never be sent back.
	return &http.Cookie{ //nolint:gosec // G124: Secure is set whenever the request is HTTPS
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   clientinfo.FromContext(ctx).HTTPS,
	}
}
