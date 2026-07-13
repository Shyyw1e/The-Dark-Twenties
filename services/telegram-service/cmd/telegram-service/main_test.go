package main

import "testing"

func TestRedactToken(t *testing.T) {
	token := "secret-token"
	value := "failed with secret-token in url"

	got := redactToken(value, token)
	if got != "failed with [REDACTED] in url" {
		t.Fatalf("redactToken() = %q", got)
	}

	if got := redactToken(value, ""); got != value {
		t.Fatalf("redactToken(empty token) = %q, want original", got)
	}
}
