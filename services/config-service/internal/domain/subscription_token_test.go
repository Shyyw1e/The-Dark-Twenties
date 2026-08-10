package domain

import (
	"testing"
	"time"
)

func TestSubscriptionTokenIsActiveAt(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name  string
		token SubscriptionToken
		want  bool
	}{
		{
			name:  "active without expiration",
			token: SubscriptionToken{Status: TokenStatusActive},
			want:  true,
		},
		{
			name:  "active before expiration",
			token: SubscriptionToken{Status: TokenStatusActive, ExpiresAt: &future},
			want:  true,
		},
		{
			name:  "active after expiration",
			token: SubscriptionToken{Status: TokenStatusActive, ExpiresAt: &past},
			want:  false,
		},
		{
			name:  "revoked",
			token: SubscriptionToken{Status: TokenStatusRevoked, ExpiresAt: &future},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsActiveAt(now); got != tt.want {
				t.Fatalf("IsActiveAt() = %v, want %v", got, tt.want)
			}
		})
	}
}
