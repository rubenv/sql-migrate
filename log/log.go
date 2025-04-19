package log

import (
	"context"
	"fmt"
)

// Level is a type to represent the log level
type Level int

const (
	LevelSilent Level = iota
	LevelError
	LevelWarn
	LevelInfo
)

type DefaultLogger struct {
	level  Level
	writer Writer
}

type Writer interface {
	Printf(string, ...interface{})
}

// DefaultLogWriter is a default implementation of the Writer interface
// that writes using fmt.Printf
type DefaultLogWriter struct{}

func (w *DefaultLogWriter) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// NewDefaultLogger creates a new DefaultLogger with a silent log level
func NewDefaultLogger() *DefaultLogger {
	return &DefaultLogger{
		level:  LevelSilent,
		writer: &DefaultLogWriter{},
	}
}

func (l *DefaultLogger) WithLevel(level Level) *DefaultLogger {
	l.level = level
	return l
}

func (l *DefaultLogger) WithWriter(writer Writer) *DefaultLogger {
	l.writer = writer
	return l
}

func (l *DefaultLogger) Info(_ context.Context, format string, args ...interface{}) {
	l.logIfPermittedByLevel(LevelInfo, format, args...)
}

func (l *DefaultLogger) Warn(_ context.Context, format string, args ...interface{}) {
	l.logIfPermittedByLevel(LevelWarn, format, args...)
}

func (l *DefaultLogger) Error(_ context.Context, format string, args ...interface{}) {
	l.logIfPermittedByLevel(LevelError, format, args...)
}

func (l *DefaultLogger) logIfPermittedByLevel(requiredLevel Level, format string, args ...interface{}) {
	if l.level < requiredLevel {
		return
	}
	l.writer.Printf(format, args...)
}
