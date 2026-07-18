package processed

import (
	"context"
	"time"
)

const StatusProcessed = "processed"

type Record struct {
	MessageID    string
	MessageType  string
	ConsumerName string
	ProcessedAt  time.Time
	Status       string
	Error        string
}

type Store interface {
	IsProcessed(ctx context.Context, messageID, consumerName string) (bool, error)
	MarkProcessed(ctx context.Context, record Record) error
}
