package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/processed"
)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		db:  db,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (s *Store) IsProcessed(ctx context.Context, messageID, consumerName string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("processed messages db is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(messageID) == "" {
		return false, errors.New("message_id is required")
	}
	if strings.TrimSpace(consumerName) == "" {
		return false, errors.New("consumer_name is required")
	}

	const query = `
SELECT EXISTS (
	SELECT 1
	FROM processed_messages
	WHERE message_id = $1
	  AND consumer_name = $2
	  AND status = $3
)`

	var exists bool
	if err := s.db.QueryRowContext(ctx, query, messageID, consumerName, processed.StatusProcessed).Scan(&exists); err != nil {
		return false, fmt.Errorf("check processed message: %w", err)
	}

	return exists, nil
}

func (s *Store) MarkProcessed(ctx context.Context, record processed.Record) error {
	if s == nil || s.db == nil {
		return errors.New("processed messages db is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(record.MessageID) == "" {
		return errors.New("message_id is required")
	}
	if strings.TrimSpace(record.ConsumerName) == "" {
		return errors.New("consumer_name is required")
	}
	if strings.TrimSpace(record.MessageType) == "" {
		return errors.New("message_type is required")
	}
	if record.ProcessedAt.IsZero() {
		record.ProcessedAt = s.now()
	}
	if strings.TrimSpace(record.Status) == "" {
		record.Status = processed.StatusProcessed
	}

	const query = `
INSERT INTO processed_messages (
	message_id,
	consumer_name,
	message_type,
	processed_at,
	status,
	error
) VALUES (
	$1, $2, $3, $4, $5, NULLIF($6, '')
)
ON CONFLICT (message_id, consumer_name)
DO UPDATE SET
	message_type = EXCLUDED.message_type,
	processed_at = EXCLUDED.processed_at,
	status = EXCLUDED.status,
	error = EXCLUDED.error`

	if _, err := s.db.ExecContext(
		ctx,
		query,
		record.MessageID,
		record.ConsumerName,
		record.MessageType,
		record.ProcessedAt,
		record.Status,
		record.Error,
	); err != nil {
		return fmt.Errorf("mark processed message: %w", err)
	}

	return nil
}
