package core

import "log"

// Logger abstracts structured and diagnostic logging to avoid package-level init() side effects.
type Logger interface {
	Printf(format string, v ...any)
	Println(v ...any)
}

// NoOpLogger drops all logs silently. Ideal for tests and CLI silent modes.
type NoOpLogger struct{}

// NewNoOpLogger returns a logger that discards all log events.
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

func (l *NoOpLogger) Printf(format string, v ...any) {}
func (l *NoOpLogger) Println(v ...any)               {}

// StdLogger wraps standard library *log.Logger.
type StdLogger struct {
	logger *log.Logger
}

// NewStdLogger wraps an existing *log.Logger.
func NewStdLogger(l *log.Logger) *StdLogger {
	return &StdLogger{logger: l}
}

func (l *StdLogger) Printf(format string, v ...any) {
	if l != nil && l.logger != nil {
		l.logger.Printf(format, v...)
	}
}

func (l *StdLogger) Println(v ...any) {
	if l != nil && l.logger != nil {
		l.logger.Println(v...)
	}
}
