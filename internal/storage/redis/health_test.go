package redis

import (
	"context"
	"strings"
	"testing"
)

func TestHealthCheckValidatesClient(t *testing.T) {
	check := NewRedisHealthCheck(nil)

	if check.Name() != "redis" {
		t.Fatalf("name = %q, want redis", check.Name())
	}
	err := check.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "client is nil") {
		t.Fatalf("error = %v, want client validation", err)
	}
}
