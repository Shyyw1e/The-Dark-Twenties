package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestHealthCheckValidatesDB(t *testing.T) {
	check := NewHealthCheck(nil)

	if check.Name() != "postgres" {
		t.Fatalf("name = %q, want postgres", check.Name())
	}
	err := check.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "db is nil") {
		t.Fatalf("error = %v, want db validation", err)
	}
}
