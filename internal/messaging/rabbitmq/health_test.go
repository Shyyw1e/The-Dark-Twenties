package rabbitmq

import (
	"context"
	"strings"
	"testing"
)

func TestHealthCheckValidatesConnection(t *testing.T) {
	err := NewHealthCheck(nil).Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "connection is nil") {
		t.Fatalf("error = %v, want connection validation", err)
	}
}
