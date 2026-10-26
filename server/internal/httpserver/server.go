// Package httpserver assembles the HTTP handler: API, embedded web app, middleware.
package httpserver

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/gh-prim/plaid/server/internal/api"
)

// New returns the root handler: /api/v1/* is the generated API, any other
// /api/* is a JSON 404, everything else is the web app.
func New(h api.StrictServerInterface, web fs.FS, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	badRequest := func(w http.ResponseWriter, _ *http.Request, err error) {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
	}
	internal := func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("handler error", "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
	strict := api.NewStrictHandlerWithOptions(h, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequest,
		ResponseErrorHandlerFunc: internal,
	})
	api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseURL:          "/api/v1",
		BaseRouter:       mux,
		ErrorHandlerFunc: badRequest,
	})

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no API route for "+r.Method+" "+r.URL.Path)
	})
	mux.Handle("/", spaHandler(web))

	return withRecover(logger, withAccessLog(logger, mux))
}
