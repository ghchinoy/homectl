//go:build !embed_ui

// Package ui provides optional embedded Web UI assets for standalone homectl builds.
package ui

import "io/fs"

// DistFS returns false when compiled without -tags embed_ui.
func DistFS() (fs.FS, bool) {
	return nil, false
}
