package rabbitmq

import (
	"context"
	"strings"
	"testing"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/message"
)

func TestPublisherValidatesConnection(t *testing.T) {
	publisher := NewPublisher(nil, nil)
	envelope, err := message.NewEnvelope(message.NewEnvelopeParams{
		MessageType:    "test.event",
		Producer:       "test",
		IdempotencyKey: "test:1",
		Payload:        map[string]string{"id": "1"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope returned error: %v", err)
	}

	err = publisher.PublishEvent(context.Background(), envelope, "test.event")
	if err == nil || !strings.Contains(err.Error(), "connection is nil") {
		t.Fatalf("error = %v, want connection validation", err)
	}
}
