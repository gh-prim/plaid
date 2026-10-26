package httpserver

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const notBuiltPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>PLAID</title></head>
<body style="background:#14151f;color:#f3ead8;font-family:system-ui;padding:3rem">
<h1>PLAID is running</h1>
<p>The web app isn't embedded in this build. Run <code>make build</code>, or use <code>make dev</code> and open the Vite dev server.</p>
</body></html>`

// spaHandler serves the built web app. Existing files are served as is;
// any other path gets index.html so client-side routing works on reload.
func spaHandler(web fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(web, name); err == nil && !st.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Vite puts content hashes in asset file names.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				http.ServeFileFS(w, r, web, name)
				return
			}
		}

		index, err := fs.ReadFile(web, "index.html")
		if err != nil {
			index = []byte(notBuiltPage)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
