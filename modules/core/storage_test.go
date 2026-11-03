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
		t.Fatalf("s.ReadFile(\"nonexistent.json\") err = %v, want %v", err, fs.ErrNotExist)
	}

	// Write and read file
	content := []byte(`{"hello":"world"}`)
	if err := s.WriteFile("test.json", content, 0644); err != nil {
		t.Fatalf("s.WriteFile(\"test.json\") failed: %v", err)
	}

	read, err := s.ReadFile("test.json")
	if err != nil {
		t.Fatalf("s.ReadFile(\"test.json\") failed: %v", err)
	}
	if string(read) != string(content) {
		t.Fatalf("s.ReadFile(\"test.json\") = %q, want %q", read, content)
	}

	// Path test
	if path := s.Path("test.json"); path != "/mem/test.json" {
		t.Fatalf("s.Path(\"test.json\") = %q, want %q", path, "/mem/test.json")
	}
}

func TestDirStorage(t *testing.T) {
	tempDir := t.TempDir()
	s := NewDirStorage(tempDir)

	if err := s.EnsureDir(); err != nil {
		t.Fatalf("s.EnsureDir() failed: %v", err)
	}

	data := []byte("hello disk")
	if err := s.WriteFile("test.txt", data, 0644); err != nil {
		t.Fatalf("s.WriteFile(\"test.txt\") failed: %v", err)
	}

	read, err := s.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("s.ReadFile(\"test.txt\") failed: %v", err)
	}
	if string(read) != string(data) {
		t.Fatalf("s.ReadFile(\"test.txt\") = %q, want %q", read, data)
	}

	expectedPath := filepath.Join(tempDir, "test.txt")
	if got := s.Path("test.txt"); got != expectedPath {
		t.Fatalf("s.Path(\"test.txt\") = %q, want %q", got, expectedPath)
	}
}

func TestXDGStorage(t *testing.T) {
	s := NewXDGStorage("homectl-test")
	dir := s.Dir()
	if dir == "" {
		t.Fatalf("s.Dir() = %q, want non-empty XDG dir", dir)
	}
	if got, want := s.Path("config.json"), filepath.Join(dir, "config.json"); got != want {
		t.Fatalf("s.Path(\"config.json\") = %q, want %q", got, want)
	}
}
