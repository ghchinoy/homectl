package sonos

import (
	"github.com/ghchinoy/homectl/modules/core"
)

var (
	defaultLogger   core.Logger   = core.NewNoOpLogger()
	defaultStorage  core.Storage  = core.NewXDGStorage("homectl")
	defaultSettings core.Settings = core.NewStaticSettings("", "")
)

// SetDefaultLogger sets the package-level logger.
func SetDefaultLogger(l core.Logger) {
	if l == nil {
		defaultLogger = core.NewNoOpLogger()
		return
	}
	defaultLogger = l
}

// DefaultLogger returns the package-level logger.
func DefaultLogger() core.Logger {
	return defaultLogger
}

// GetDefaultLogger returns the package-level logger.
// Deprecated: use DefaultLogger instead.
func GetDefaultLogger() core.Logger {
	return DefaultLogger()
}

// SetDefaultStorage sets the package-level storage provider for caching.
func SetDefaultStorage(s core.Storage) {
	if s == nil {
		defaultStorage = core.NewXDGStorage("homectl")
		return
	}
	defaultStorage = s
}

// DefaultStorage returns the package-level storage provider.
func DefaultStorage() core.Storage {
	return defaultStorage
}

// GetDefaultStorage returns the package-level storage provider.
// Deprecated: use DefaultStorage instead.
func GetDefaultStorage() core.Storage {
	return DefaultStorage()
}

// SetDefaultSettings sets the package-level settings provider.
func SetDefaultSettings(s core.Settings) {
	if s == nil {
		defaultSettings = core.NewStaticSettings("", "")
		return
	}
	defaultSettings = s
}

// DefaultSettings returns the package-level settings provider.
func DefaultSettings() core.Settings {
	return defaultSettings
}

// GetDefaultSettings returns the package-level settings provider.
// Deprecated: use DefaultSettings instead.
func GetDefaultSettings() core.Settings {
	return DefaultSettings()
}
