//go:build embed_ui

// Package ui provides optional embedded Web UI assets for standalone homectl builds.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedDist embed.FS

// DistFS returns the embedded ui/dist filesystem when built with -tags embed_ui
// and index.html is present in the bundle.
func DistFS() (fs.FS, bool) {
	sub, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, false
	}
	return sub, true
}
