package httpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
)

func TestServiceValidatesAddress(t *testing.T) {
	service := NewService(config.HTTPConfig{}, nil, nil)

	if service.Name() != defaultServiceName {
		t.Fatalf("name = %q, want %q", service.Name(), defaultServiceName)
	}
	err := service.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "address is empty") {
		t.Fatalf("error = %v, want address validation", err)
	}
}
