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

// GetDefaultLogger returns the package-level logger.
func GetDefaultLogger() core.Logger {
	return defaultLogger
}

// SetDefaultStorage sets the package-level storage provider for caching.
func SetDefaultStorage(s core.Storage) {
	if s == nil {
		defaultStorage = core.NewXDGStorage("homectl")
		return
	}
	defaultStorage = s
}

// GetDefaultStorage returns the package-level storage provider.
func GetDefaultStorage() core.Storage {
	return defaultStorage
}

// SetDefaultSettings sets the package-level settings provider.
func SetDefaultSettings(s core.Settings) {
	if s == nil {
		defaultSettings = core.NewStaticSettings("", "")
		return
	}
	defaultSettings = s
}

// GetDefaultSettings returns the package-level settings provider.
func GetDefaultSettings() core.Settings {
	return defaultSettings
}
