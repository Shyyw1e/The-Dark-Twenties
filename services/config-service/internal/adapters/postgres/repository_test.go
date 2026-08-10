package postgres

import (
	"database/sql"
	"testing"
	"time"
)

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			*target = r.values[i].(string)
		case *int:
			*target = r.values[i].(int)
		case *int64:
			*target = r.values[i].(int64)
		case *time.Time:
			*target = r.values[i].(time.Time)
		case *sql.NullString:
			*target = r.values[i].(sql.NullString)
		case *sql.NullTime:
			*target = r.values[i].(sql.NullTime)
		default:
			panic("unsupported scan destination")
		}
	}
	return nil
}

func TestScanSubscriptionToken(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)

	token, err := scanSubscriptionToken(fakeRow{values: []any{
		"token-1",
		"user-1",
		"device-1",
		sql.NullString{String: "subscription-1", Valid: true},
		"hash",
		"active",
		"happ",
		"sing-box",
		sql.NullTime{Time: expiresAt, Valid: true},
		sql.NullTime{Time: now, Valid: true},
		sql.NullString{String: "Happ/1.0", Valid: true},
		int64(7),
		sql.NullTime{},
		now,
		now,
	}})
	if err != nil {
		t.Fatalf("scanSubscriptionToken returned error: %v", err)
	}

	if token.ID != "token-1" || token.SubscriptionID != "subscription-1" || token.LastUserAgent != "Happ/1.0" {
		t.Fatalf("token = %+v", token)
	}
	if token.ExpiresAt == nil || !token.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expires_at = %v", token.ExpiresAt)
	}
	if token.LastUsedAt == nil || !token.LastUsedAt.Equal(now) {
		t.Fatalf("last_used_at = %v", token.LastUsedAt)
	}
	if token.RevokedAt != nil {
		t.Fatalf("revoked_at = %v, want nil", token.RevokedAt)
	}
}

func TestScanConfigProfile(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)

	profile, err := scanConfigProfile(fakeRow{values: []any{
		"profile-1",
		"token-1",
		3,
		"happ",
		"sing-box",
		2,
		`{"outbounds":[]}`,
		"content-hash",
		`["node-1"]`,
		`{"region":"nl"}`,
		now,
		sql.NullTime{Time: expiresAt, Valid: true},
	}})
	if err != nil {
		t.Fatalf("scanConfigProfile returned error: %v", err)
	}

	if profile.ID != "profile-1" || profile.Content != `{"outbounds":[]}` || profile.NodeRefs != `["node-1"]` {
		t.Fatalf("profile = %+v", profile)
	}
	if profile.ExpiresAt == nil || !profile.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expires_at = %v", profile.ExpiresAt)
	}
}

func TestNullString(t *testing.T) {
	if got := nullString(" value "); !got.Valid || got.String != "value" {
		t.Fatalf("nullString filled = %+v", got)
	}
	if got := nullString("   "); got.Valid || got.String != "" {
		t.Fatalf("nullString blank = %+v", got)
	}
}
