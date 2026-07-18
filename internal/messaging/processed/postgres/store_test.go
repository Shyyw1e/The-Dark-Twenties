package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/processed"
)

func TestStoreValidatesDB(t *testing.T) {
	store := NewStore(nil)

	_, err := store.IsProcessed(context.Background(), "message-id", "consumer")
	if err == nil || !strings.Contains(err.Error(), "db is nil") {
		t.Fatalf("IsProcessed error = %v, want db validation", err)
	}

	err = store.MarkProcessed(context.Background(), processed.Record{
		MessageID:    "message-id",
		MessageType:  "payment.succeeded",
		ConsumerName: "consumer",
	})
	if err == nil || !strings.Contains(err.Error(), "db is nil") {
		t.Fatalf("MarkProcessed error = %v, want db validation", err)
	}
}

func TestStoreValidatesRecord(t *testing.T) {
	store := &Store{}

	err := store.MarkProcessed(context.Background(), processed.Record{})
	if err == nil {
		t.Fatal("expected error")
	}
}
