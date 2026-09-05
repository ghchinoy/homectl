package core

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Storage abstracts persistence of configuration files, cache, certificates, and state.
type Storage interface {
	// Path returns the absolute path for a filename within the storage area.
	Path(filename string) string

	// EnsureDir ensures that the storage directory exists.
	EnsureDir() error

	// ReadFile reads the named file from the storage area.
	ReadFile(filename string) ([]byte, error)

	// WriteFile writes data to the named file within the storage area.
	WriteFile(filename string, data []byte, perm os.FileMode) error
}

// XDGStorage implements Storage adhering to the XDG Base Directory specification.
type XDGStorage struct {
	appName string
}

// NewXDGStorage returns a new XDGStorage for the given application name.
func NewXDGStorage(appName string) *XDGStorage {
	if appName == "" {
		appName = "homectl"
	}
	return &XDGStorage{appName: appName}
}

// Dir returns the root directory for this XDGStorage instance.
func (s *XDGStorage) Dir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".config", s.appName)
	}
	return filepath.Join(configDir, s.appName)
}

func (s *XDGStorage) Path(filename string) string {
	return filepath.Join(s.Dir(), filename)
}

func (s *XDGStorage) EnsureDir() error {
	return os.MkdirAll(s.Dir(), 0755)
}

func (s *XDGStorage) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(s.Path(filename))
}

func (s *XDGStorage) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	return os.WriteFile(s.Path(filename), data, perm)
}

// DirStorage implements Storage rooted at an explicit directory.
type DirStorage struct {
	dir string
}

// NewDirStorage returns a Storage rooted at the specified directory path.
func NewDirStorage(dir string) *DirStorage {
	return &DirStorage{dir: dir}
}

func (s *DirStorage) Path(filename string) string {
	return filepath.Join(s.dir, filename)
}

func (s *DirStorage) EnsureDir() error {
	return os.MkdirAll(s.dir, 0755)
}

func (s *DirStorage) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(s.Path(filename))
}

func (s *DirStorage) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	return os.WriteFile(s.Path(filename), data, perm)
}

// MemoryStorage implements in-memory Storage for tests and ephemeral agent runs.
type MemoryStorage struct {
	mu    sync.RWMutex
	files map[string][]byte
}

// NewMemoryStorage creates an in-memory Storage instance.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		files: make(map[string][]byte),
	}
}

func (s *MemoryStorage) Path(filename string) string {
	return "/mem/" + filename
}

func (s *MemoryStorage) EnsureDir() error {
	return nil
}

func (s *MemoryStorage) ReadFile(filename string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.files[filename]
	if !ok {
		return nil, fs.ErrNotExist
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

func (s *MemoryStorage) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if filename == "" {
		return errors.New("empty filename")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	s.files[filename] = cp
	return nil
}
