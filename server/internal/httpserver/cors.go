package httpserver

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gh-prim/plaid/server/internal/auth"
)

// withCORS lets the web app hosted on getplaid.direct (the server's short
// link, F012) call this server from the browser. Only the listed origins get
// CORS headers, with credentials: the hosted app and the server share a site
// (getplaid.direct), so the session cookie works across them. Preflights
// asking for private network access (a public page reaching a LAN address)
// are allowed too.
func withCORS(origins []string, next http.Handler) http.Handler {
	if len(origins) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || !slices.Contains(origins, origin) {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Add("Vary", "Origin")
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Access-Control-Expose-Headers", "Retry-After, Content-Range, Content-Length, X-Subtitle-Until")
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE")
			if req := r.Header.Get("Access-Control-Request-Headers"); req != "" {
				h.Set("Access-Control-Allow-Headers", req)
			}
			if strings.EqualFold(r.Header.Get("Access-Control-Request-Private-Network"), "true") {
				h.Set("Access-Control-Allow-Private-Network", "true")
			}
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithAppOrigin(r.Context())))
	})
}
