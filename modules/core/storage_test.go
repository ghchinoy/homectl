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
		t.Fatalf("ReadFile(%q) error = %v, want %v", "nonexistent.json", err, fs.ErrNotExist)
	}

	// Write and read file
	content := []byte(`{"hello":"world"}`)
	if err := s.WriteFile("test.json", content, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	read, err := s.ReadFile("test.json")
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", "test.json", err)
	}
	if string(read) != string(content) {
		t.Fatalf("ReadFile(%q) = %q, want %q", "test.json", string(read), string(content))
	}

	// Path test
	if path := s.Path("test.json"); path != "/mem/test.json" {
		t.Fatalf("Path(%q) = %q, want %q", "test.json", path, "/mem/test.json")
	}
}

func TestDirStorage(t *testing.T) {
	tempDir := t.TempDir()
	s := NewDirStorage(tempDir)

	if err := s.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir() error = %v, want nil", err)
	}

	data := []byte("hello disk")
	if err := s.WriteFile("test.txt", data, 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", "test.txt", err)
	}

	read, err := s.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", "test.txt", err)
	}
	if string(read) != string(data) {
		t.Fatalf("ReadFile(%q) = %q, want %q", "test.txt", string(read), string(data))
	}

	expectedPath := filepath.Join(tempDir, "test.txt")
	if got := s.Path("test.txt"); got != expectedPath {
		t.Fatalf("Path(%q) = %q, want %q", "test.txt", got, expectedPath)
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
