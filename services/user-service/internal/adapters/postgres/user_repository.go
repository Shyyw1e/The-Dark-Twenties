package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("user repository db is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	const query = `
SELECT
	id,
	telegram_id,
	username,
	first_name,
	last_name,
	language_code,
	status,
	is_admin,
	blocked_at,
	created_at,
	updated_at
FROM users
WHERE telegram_id = $1
LIMIT 1`

	user, err := scanUser(r.db.QueryRowContext(ctx, query, telegramID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by telegram_id: %w", err)
	}

	return user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	if r == nil || r.db == nil {
		return errors.New("user repository db is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if user == nil {
		return errors.New("user is nil")
	}

	const query = `
INSERT INTO users (
	id,
	telegram_id,
	username,
	first_name,
	last_name,
	language_code,
	status,
	is_admin,
	blocked_at,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)`

	if _, err := r.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.TelegramID,
		nullString(user.Username),
		nullString(user.FirstName),
		nullString(user.LastName),
		nullString(user.LanguageCode),
		string(user.Status),
		user.IsAdmin,
		user.BlockedAt,
		user.CreatedAt,
		user.UpdatedAt,
	); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

func (r *UserRepository) UpdateTelegramProfile(ctx context.Context, user *domain.User) error {
	if r == nil || r.db == nil {
		return errors.New("user repository db is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if user == nil {
		return errors.New("user is nil")
	}

	const query = `
UPDATE users
SET
	username = $2,
	first_name = $3,
	last_name = $4,
	language_code = $5,
	updated_at = $6
WHERE id = $1`

	result, err := r.db.ExecContext(
		ctx,
		query,
		user.ID,
		nullString(user.Username),
		nullString(user.FirstName),
		nullString(user.LastName),
		nullString(user.LanguageCode),
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update telegram profile: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update telegram profile rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*domain.User, error) {
	var user domain.User
	var username sql.NullString
	var firstName sql.NullString
	var lastName sql.NullString
	var languageCode sql.NullString
	var blockedAt sql.NullTime
	var status string

	if err := row.Scan(
		&user.ID,
		&user.TelegramID,
		&username,
		&firstName,
		&lastName,
		&languageCode,
		&status,
		&user.IsAdmin,
		&blockedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return nil, err
	}

	user.Username = username.String
	user.FirstName = firstName.String
	user.LastName = lastName.String
	user.LanguageCode = languageCode.String
	user.Status = domain.Status(status)
	if blockedAt.Valid {
		value := blockedAt.Time
		user.BlockedAt = &value
	}

	return &user, nil
}

func nullString(value string) sql.NullString {
	return sql.NullString{
		String: value,
		Valid:  value != "",
	}
}
