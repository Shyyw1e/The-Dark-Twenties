package logger

import (
	"context"
	"testing"
)

func TestContextLogger(t *testing.T) {
	base := context.Background()
	log := &nopLogger{}

	ctx := IntoContext(base, log)
	if got := FromContext(ctx); got != log {
		t.Fatalf("FromContext = %T, want provided logger", got)
	}

	if got := FromContext(nil); got == nil {
		t.Fatal("FromContext(nil) returned nil")
	}

	if got := IntoContext(base, nil); got != base {
		t.Fatal("IntoContext with nil logger should return original context")
	}
}
