package core

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
)

func TestMemoryStorage(t *testing.T) {
	s := NewMemoryStorage()

	// EnsureDir should succeed
	if err := s.EnsureDir(); err != nil {
		t.Fatalf("unexpected error from EnsureDir: %v", err)
	}

	// Read non-existent file
	_, err := s.ReadFile("nonexistent.json")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}

	// Write and read file
	content := []byte(`{"hello":"world"}`)
	if err := s.WriteFile("test.json", content, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	read, err := s.ReadFile("test.json")
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(read) != string(content) {
		t.Fatalf("expected %s, got %s", content, read)
	}

	// Path test
	if path := s.Path("test.json"); path != "/mem/test.json" {
		t.Fatalf("unexpected path: %s", path)
	}
}

func TestDirStorage(t *testing.T) {
	tempDir := t.TempDir()
	s := NewDirStorage(tempDir)

	if err := s.EnsureDir(); err != nil {
		t.Fatalf("failed to ensure dir: %v", err)
	}

	data := []byte("hello disk")
	if err := s.WriteFile("test.txt", data, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	read, err := s.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(read) != string(data) {
		t.Fatalf("expected %s, got %s", data, read)
	}

	expectedPath := filepath.Join(tempDir, "test.txt")
	if s.Path("test.txt") != expectedPath {
		t.Fatalf("expected path %s, got %s", expectedPath, s.Path("test.txt"))
	}
}

func TestXDGStorage(t *testing.T) {
	s := NewXDGStorage("homectl-test")
	dir := s.Dir()
	if dir == "" {
		t.Fatalf("expected non-empty XDG dir")
	}
	if s.Path("config.json") != filepath.Join(dir, "config.json") {
		t.Fatalf("path does not match XDG dir")
	}
}
