// Package webapp embeds the PLAID web app built for getplaid.direct (Vite
// base /app/, hosted mode), served on each server's short link. The Docker
// build copies it into dist/; without it dist/ only holds .keep and short
// links redirect to the server instead.
package webapp

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

// FS returns the built app, rooted at dist/.
func FS() fs.FS {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err) // dist/ is embedded at compile time, so this can't happen
	}
	return sub
}

// Built reports whether the app is in the binary.
func Built() bool {
	_, err := fs.Stat(FS(), "index.html")
	return err == nil
}
