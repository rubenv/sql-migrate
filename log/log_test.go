package log_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/rubenv/sql-migrate/log"
)

type mockWriter struct {
	logs []string
}

func (mw *mockWriter) Printf(format string, args ...interface{}) {
	mw.logs = append(mw.logs, fmt.Sprintf(format, args...))
}

func TestDefaultLoggerWithLevelInfo(t *testing.T) {
	mockWriter := &mockWriter{logs: []string{}}

	logger := log.NewDefaultLogger().WithLevel(log.LevelInfo).WithWriter(mockWriter)
	logger.Info(context.Background(), "This should be logged")
	logger.Warn(context.Background(), "This should also be logged")
	logger.Error(context.Background(), "This should also be logged")

	expectedLogs := []string{
		"This should be logged",
		"This should also be logged",
		"This should also be logged",
	}

	if len(mockWriter.logs) != len(expectedLogs) {
		t.Fatalf("Expected %d logs, got %d", len(expectedLogs), len(mockWriter.logs))
	}

	for i, expectedLog := range expectedLogs {
		if expectedLog != mockWriter.logs[i] {
			t.Fatalf("Expected log %d to be %s, got %s", i, expectedLog, mockWriter.logs[i])
		}
	}
}

func TestDefaultLoggerWithLevelSilent(t *testing.T) {
	mockWriter := &mockWriter{logs: []string{}}

	logger := log.NewDefaultLogger().WithLevel(log.LevelSilent).WithWriter(mockWriter)
	logger.Info(context.Background(), "This should not be logged")
	logger.Warn(context.Background(), "This should not be logged")
	logger.Error(context.Background(), "This should not be logged")

	if len(mockWriter.logs) != 0 {
		t.Fatalf("Expected no logs, got %d", len(mockWriter.logs))
	}
}

func TestDefaultLoggerWithLevelWarn(t *testing.T) {
	mockWriter := &mockWriter{logs: []string{}}

	logger := log.NewDefaultLogger().WithLevel(log.LevelWarn).WithWriter(mockWriter)
	logger.Info(context.Background(), "This should not be logged")
	logger.Warn(context.Background(), "This should be logged")
	logger.Error(context.Background(), "This should also be logged")

	expectedLogs := []string{
		"This should be logged",
		"This should also be logged",
	}

	if len(mockWriter.logs) != len(expectedLogs) {
		t.Fatalf("Expected %d logs, got %d", len(expectedLogs), len(mockWriter.logs))
	}

	for i, expectedLog := range expectedLogs {
		if expectedLog != mockWriter.logs[i] {
			t.Fatalf("Expected log %d to be %s, got %s", i, expectedLog, mockWriter.logs[i])
		}
	}
}
