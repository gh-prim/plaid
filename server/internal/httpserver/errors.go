package httpserver

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gh-prim/plaid/server/internal/apierr"
)

// writeError writes the shared API error shape: {"error": {"code", "message"}}.
// code is a stable key clients translate; message is English, for developers.
func writeError(w http.ResponseWriter, status int, code, message string) {
	apierr.Write(w, apierr.New(status, code, message))
}

// handlerErrorFunc writes an *apierr.Error returned by a handler as is.
// Anything else is unexpected: it's logged and becomes a 500.
func handlerErrorFunc(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		if e, ok := errors.AsType[*apierr.Error](err); ok {
			apierr.Write(w, e)
			return
		}
		logger.Error("handler error", "path", r.URL.Path, "err", err)
		apierr.Write(w, apierr.Internal())
	}
}
