package grpcserver

import (
	"context"
	"strings"
	"testing"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
)

func TestServiceValidatesServer(t *testing.T) {
	service := NewService(config.GRPCConfig{Addr: ":0"}, nil, nil)

	if service.Name() != defaultServiceName {
		t.Fatalf("name = %q, want %q", service.Name(), defaultServiceName)
	}
	err := service.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "server is nil") {
		t.Fatalf("error = %v, want server validation", err)
	}
}
