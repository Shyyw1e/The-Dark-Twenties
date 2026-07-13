package pprof

import (
	"context"
	"testing"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
)

func TestDisabledService(t *testing.T) {
	service := NewService(config.RuntimeConfig{PprofEnabled: false, PprofAddr: "127.0.0.1:0"}, nil)

	if service.Name() != serviceName {
		t.Fatalf("name = %q, want %q", service.Name(), serviceName)
	}
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}
