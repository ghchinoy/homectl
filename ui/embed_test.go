//go:build embed_ui

package ui

import (
	"io/fs"
	"testing"
)

func TestDistFS_EmbeddedBundleContainsIndexHTML(t *testing.T) {
	sub, ok := DistFS()
	if !ok || sub == nil {
		t.Fatal("DistFS() = (_, false), want embedded ui/dist filesystem")
	}
	data, err := fs.ReadFile(sub, "index.html")
	if err != nil || len(data) == 0 {
		t.Fatalf("fs.ReadFile(sub, index.html) = (%d bytes, %v), want non-empty index.html", len(data), err)
	}
}
