package domain

import (
	"testing"
	"time"
)

func TestUserStatusHelpers(t *testing.T) {
	user := User{Status: StatusActive}
	if user.IsBlocked() {
		t.Fatal("active user must not be blocked")
	}
	if user.IsDeleted() {
		t.Fatal("active user must not be deleted")
	}

	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	user.Block(now)
	if !user.IsBlocked() {
		t.Fatal("blocked user must be blocked")
	}
	if user.BlockedAt == nil || !user.BlockedAt.Equal(now) {
		t.Fatalf("blocked_at = %v, want %v", user.BlockedAt, now)
	}

	user.Activate()
	if user.IsBlocked() {
		t.Fatal("activated user must not be blocked")
	}
	if user.BlockedAt != nil {
		t.Fatalf("activated user blocked_at = %v, want nil", user.BlockedAt)
	}

	user.Delete()
	if !user.IsDeleted() {
		t.Fatal("deleted user must be deleted")
	}
}
