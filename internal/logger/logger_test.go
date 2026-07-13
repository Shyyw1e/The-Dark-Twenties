package logger

import "testing"

func TestNewLogger(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "warning", "error", "unknown", ""} {
		t.Run(level, func(t *testing.T) {
			log := New(level, "test-service")
			if log == nil {
				t.Fatal("logger is nil")
			}
			if child := log.With("key", "value"); child == nil {
				t.Fatal("child logger is nil")
			}
		})
	}
}
