package rabbitmq

import (
	"context"
	"strings"
	"testing"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/message"
)

func TestConsumerDefaultsAndValidation(t *testing.T) {
	consumer := NewConsumer(nil, nil, ConsumerOptions{Queue: "queue"}, HandlerFunc(func(ctx context.Context, envelope *message.Envelope) error {
		return nil
	}))

	if consumer.Name() != "rabbitmq-consumer" {
		t.Fatalf("name = %q, want rabbitmq-consumer", consumer.Name())
	}
	if consumer.opts.PrefetchCount != defaultPrefetchCount {
		t.Fatalf("prefetch = %d, want %d", consumer.opts.PrefetchCount, defaultPrefetchCount)
	}
	if consumer.opts.ConsumerTag != "queue" {
		t.Fatalf("consumer tag = %q, want queue", consumer.opts.ConsumerTag)
	}

	err := consumer.validate()
	if err == nil || !strings.Contains(err.Error(), "connection is nil") {
		t.Fatalf("error = %v, want connection validation", err)
	}
}
