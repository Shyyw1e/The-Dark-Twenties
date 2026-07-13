package runtimeobs

import (
	"context"
	"testing"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
)

func TestDisabledMonitor(t *testing.T) {
	monitor := NewMonitor(config.RuntimeConfig{
		MonitorEnabled:  false,
		MonitorInterval: time.Millisecond,
	}, nil)

	if monitor.Name() != serviceName {
		t.Fatalf("name = %q, want %q", monitor.Name(), serviceName)
	}
	if err := monitor.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := monitor.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}
