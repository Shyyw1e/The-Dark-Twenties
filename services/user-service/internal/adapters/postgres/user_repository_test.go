package postgres

import (
	"database/sql"
	"testing"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/domain"
)

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *string:
			*target = value.(string)
		case *int64:
			*target = value.(int64)
		case *bool:
			*target = value.(bool)
		case *time.Time:
			*target = value.(time.Time)
		case *sql.NullString:
			*target = value.(sql.NullString)
		case *sql.NullTime:
			*target = value.(sql.NullTime)
		default:
			panic("unsupported scan target")
		}
	}
	return nil
}

func TestScanUser(t *testing.T) {
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	blockedAt := now.Add(-time.Hour)

	user, err := scanUser(fakeRow{values: []any{
		"user-1",
		int64(1001),
		sql.NullString{String: "shyywie", Valid: true},
		sql.NullString{String: "Shyy", Valid: true},
		sql.NullString{},
		sql.NullString{String: "ru", Valid: true},
		string(domain.StatusBlocked),
		true,
		sql.NullTime{Time: blockedAt, Valid: true},
		now,
		now,
	}})
	if err != nil {
		t.Fatalf("scanUser returned error: %v", err)
	}

	if user.ID != "user-1" || user.TelegramID != 1001 || user.Username != "shyywie" || user.Status != domain.StatusBlocked || !user.IsAdmin {
		t.Fatalf("user = %+v", user)
	}
	if user.BlockedAt == nil || !user.BlockedAt.Equal(blockedAt) {
		t.Fatalf("blocked_at = %v, want %v", user.BlockedAt, blockedAt)
	}
}

func TestNullString(t *testing.T) {
	if value := nullString("hello"); !value.Valid || value.String != "hello" {
		t.Fatalf("nullString(non-empty) = %+v", value)
	}
	if value := nullString(""); value.Valid || value.String != "" {
		t.Fatalf("nullString(empty) = %+v", value)
	}
}
