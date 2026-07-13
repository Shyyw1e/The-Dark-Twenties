package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
)

func TestNewServiceDefaults(t *testing.T) {
	service := NewService(nil, nil, config.TelegramConfig{}, nil)

	if service.Name() != serviceName {
		t.Fatalf("name = %q, want %q", service.Name(), serviceName)
	}
	if service.pollTimeout != 30 {
		t.Fatalf("poll timeout = %d, want 30", service.pollTimeout)
	}
	if service.done == nil {
		t.Fatal("done channel is nil")
	}
}

func TestStartValidatesDependencies(t *testing.T) {
	service := NewService(nil, nil, config.TelegramConfig{}, nil)

	err := service.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "telegram bot api is nil") {
		t.Fatalf("error = %v, want telegram bot api validation", err)
	}
}
