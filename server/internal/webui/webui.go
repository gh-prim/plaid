// Package webui embeds the built web app (copied into dist/ by `make build`).
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

// FS returns the built web app, rooted at dist/. It only contains .keep
// when the web app hasn't been built; the server then shows a hint page.
func FS() fs.FS {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err) // dist/ is embedded at compile time, so this can't happen
	}
	return sub
}
