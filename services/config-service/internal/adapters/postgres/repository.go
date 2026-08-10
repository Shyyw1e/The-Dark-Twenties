package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindTokenByHash(ctx context.Context, tokenHash string) (*domain.SubscriptionToken, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	const query = `
SELECT
	id,
	user_id,
	device_id,
	subscription_id,
	token_hash,
	status,
	client_type,
	format,
	expires_at,
	last_used_at,
	last_user_agent,
	refresh_count,
	revoked_at,
	created_at,
	updated_at
FROM subscription_tokens
WHERE token_hash = $1
LIMIT 1`

	token, err := scanSubscriptionToken(r.db.QueryRowContext(ctx, query, strings.TrimSpace(tokenHash)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSubscriptionTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find subscription token by hash: %w", err)
	}

	return token, nil
}

func (r *Repository) FindActiveTokenByUserID(ctx context.Context, userID string, clientType string, format string, now time.Time) (*domain.SubscriptionToken, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	const query = `
SELECT
	id,
	user_id,
	device_id,
	subscription_id,
	token_hash,
	status,
	client_type,
	format,
	expires_at,
	last_used_at,
	last_user_agent,
	refresh_count,
	revoked_at,
	created_at,
	updated_at
FROM subscription_tokens
WHERE user_id = $1
	AND client_type = $2
	AND format = $3
	AND status = 'active'
	AND (expires_at IS NULL OR expires_at > $4)
ORDER BY updated_at DESC
LIMIT 1`

	token, err := scanSubscriptionToken(r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(userID),
		strings.TrimSpace(clientType),
		strings.TrimSpace(format),
		now,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSubscriptionTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find active subscription token by user_id: %w", err)
	}

	return token, nil
}

func (r *Repository) FindLatestProfileByTokenID(ctx context.Context, tokenID string) (*domain.ConfigProfile, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	const query = `
SELECT
	id,
	subscription_token_id,
	profile_version,
	client_type,
	format,
	server_count,
	content,
	content_hash,
	node_refs::text,
	metadata::text,
	generated_at,
	expires_at
FROM config_profiles
WHERE subscription_token_id = $1
ORDER BY profile_version DESC
LIMIT 1`

	profile, err := scanConfigProfile(r.db.QueryRowContext(ctx, query, strings.TrimSpace(tokenID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrConfigProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find latest config profile by token_id: %w", err)
	}

	return profile, nil
}

func (r *Repository) CreateSubscriptionToken(ctx context.Context, token *domain.SubscriptionToken) error {
	if err := r.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if token == nil {
		return errors.New("subscription token is nil")
	}

	const query = `
INSERT INTO subscription_tokens (
	id,
	user_id,
	device_id,
	subscription_id,
	token_hash,
	status,
	client_type,
	format,
	expires_at,
	last_used_at,
	last_user_agent,
	refresh_count,
	revoked_at,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)`

	if _, err := r.db.ExecContext(
		ctx,
		query,
		token.ID,
		token.UserID,
		token.DeviceID,
		nullString(token.SubscriptionID),
		token.TokenHash,
		string(token.Status),
		token.ClientType,
		token.Format,
		nullTime(token.ExpiresAt),
		nullTime(token.LastUsedAt),
		nullString(token.LastUserAgent),
		token.RefreshCount,
		nullTime(token.RevokedAt),
		token.CreatedAt,
		token.UpdatedAt,
	); err != nil {
		return fmt.Errorf("create subscription token: %w", err)
	}

	return nil
}

func (r *Repository) CreateConfigProfile(ctx context.Context, profile *domain.ConfigProfile) error {
	if err := r.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if profile == nil {
		return errors.New("config profile is nil")
	}

	const query = `
INSERT INTO config_profiles (
	id,
	subscription_token_id,
	profile_version,
	client_type,
	format,
	server_count,
	content,
	content_hash,
	node_refs,
	metadata,
	generated_at,
	expires_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11, $12
)`

	if _, err := r.db.ExecContext(
		ctx,
		query,
		profile.ID,
		profile.SubscriptionTokenID,
		profile.ProfileVersion,
		profile.ClientType,
		profile.Format,
		profile.ServerCount,
		profile.Content,
		profile.ContentHash,
		profile.NodeRefs,
		profile.Metadata,
		profile.GeneratedAt,
		nullTime(profile.ExpiresAt),
	); err != nil {
		return fmt.Errorf("create config profile: %w", err)
	}

	return nil
}

func (r *Repository) MarkTokenUsed(ctx context.Context, tokenID string, usedAt time.Time, userAgent string) error {
	if err := r.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	const query = `
UPDATE subscription_tokens
SET
	last_used_at = $2,
	last_user_agent = $3,
	refresh_count = refresh_count + 1,
	updated_at = $2
WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, strings.TrimSpace(tokenID), usedAt, nullString(userAgent))
	if err != nil {
		return fmt.Errorf("mark subscription token used: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark subscription token used rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrSubscriptionTokenNotFound
	}

	return nil
}

func (r *Repository) CreateRefreshEvent(ctx context.Context, event *domain.RefreshEvent) error {
	if err := r.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if event == nil {
		return errors.New("refresh event is nil")
	}

	const query = `
INSERT INTO subscription_refresh_events (
	id,
	subscription_token_id,
	user_id,
	device_id,
	client_type,
	user_agent,
	source_ip_hash,
	returned_profile_version,
	returned_server_count,
	created_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)`

	if _, err := r.db.ExecContext(
		ctx,
		query,
		event.ID,
		event.SubscriptionTokenID,
		event.UserID,
		event.DeviceID,
		nullString(event.ClientType),
		nullString(event.UserAgent),
		nullString(event.SourceIPHash),
		event.ReturnedProfileVersion,
		event.ReturnedServerCount,
		event.CreatedAt,
	); err != nil {
		return fmt.Errorf("create subscription refresh event: %w", err)
	}

	return nil
}

func (r *Repository) validate() error {
	if r == nil || r.db == nil {
		return errors.New("config repository db is nil")
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSubscriptionToken(row rowScanner) (*domain.SubscriptionToken, error) {
	var token domain.SubscriptionToken
	var subscriptionID sql.NullString
	var expiresAt sql.NullTime
	var lastUsedAt sql.NullTime
	var lastUserAgent sql.NullString
	var revokedAt sql.NullTime
	var status string

	if err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.DeviceID,
		&subscriptionID,
		&token.TokenHash,
		&status,
		&token.ClientType,
		&token.Format,
		&expiresAt,
		&lastUsedAt,
		&lastUserAgent,
		&token.RefreshCount,
		&revokedAt,
		&token.CreatedAt,
		&token.UpdatedAt,
	); err != nil {
		return nil, err
	}

	token.SubscriptionID = subscriptionID.String
	token.Status = domain.TokenStatus(status)
	token.LastUserAgent = lastUserAgent.String
	if expiresAt.Valid {
		value := expiresAt.Time
		token.ExpiresAt = &value
	}
	if lastUsedAt.Valid {
		value := lastUsedAt.Time
		token.LastUsedAt = &value
	}
	if revokedAt.Valid {
		value := revokedAt.Time
		token.RevokedAt = &value
	}

	return &token, nil
}

func scanConfigProfile(row rowScanner) (*domain.ConfigProfile, error) {
	var profile domain.ConfigProfile
	var expiresAt sql.NullTime

	if err := row.Scan(
		&profile.ID,
		&profile.SubscriptionTokenID,
		&profile.ProfileVersion,
		&profile.ClientType,
		&profile.Format,
		&profile.ServerCount,
		&profile.Content,
		&profile.ContentHash,
		&profile.NodeRefs,
		&profile.Metadata,
		&profile.GeneratedAt,
		&expiresAt,
	); err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		value := expiresAt.Time
		profile.ExpiresAt = &value
	}

	return &profile, nil
}

func nullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func nullTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}
