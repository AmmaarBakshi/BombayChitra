// Package webui embeds the production build of the web app (web/ → server/internal/webui/dist) into the binary, so
// the server ships as a single file.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the embedded build, or nil when the web app was not built before compiling the server.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}
