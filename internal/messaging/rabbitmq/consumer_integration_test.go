package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/message"
	amqp "github.com/rabbitmq/amqp091-go"
)

func TestConsumerRetryLevelsAndDLQIntegration(t *testing.T) {
	if os.Getenv("RABBITMQ_SMOKE") != "1" {
		t.Skip("set RABBITMQ_SMOKE=1 to run RabbitMQ smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := &config.RabbitMQConfig{
		URL:              getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		EventsExchange:   getEnv("RABBITMQ_EVENTS_EXCHANGE", "vpn.events.topic"),
		CommandsExchange: getEnv("RABBITMQ_COMMANDS_EXCHANGE", "vpn.commands.direct"),
		RetryExchange:    getEnv("RABBITMQ_RETRY_EXCHANGE", "vpn.retry"),
		DLXExchange:      getEnv("RABBITMQ_DLX_EXCHANGE", "vpn.dlx"),
	}

	conn, err := Connect(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("connect rabbitmq: %v", err)
	}
	defer conn.Close()

	queue := fmt.Sprintf("smoke.retry.%d", time.Now().UnixNano())
	routingKey := queue + ".command"
	levels := []RetryLevel{
		{Name: "50ms", Delay: 50 * time.Millisecond},
		{Name: "100ms", Delay: 100 * time.Millisecond},
		{Name: "150ms", Delay: 150 * time.Millisecond},
	}

	cleanupQueues(t, conn, queue, levels)
	defer cleanupQueues(t, conn, queue, levels)

	var handled int32
	consumer := NewCommandConsumer(conn, nil, ConsumerOptions{
		Name:       queue,
		Queue:      queue,
		RoutingKey: routingKey,
		Retry: RetryPolicy{
			Enabled:     true,
			MaxAttempts: 3,
			Levels:      levels,
		},
	}, HandlerFunc(func(ctx context.Context, envelope *message.Envelope) error {
		atomic.AddInt32(&handled, 1)
		return errors.New("forced smoke failure")
	}))

	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start consumer: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := consumer.Stop(stopCtx); err != nil {
			t.Logf("stop consumer: %v", err)
		}
	}()

	envelope, err := message.NewEnvelope(message.NewEnvelopeParams{
		MessageType:    "smoke.retry",
		Producer:       "rabbitmq-smoke-test",
		IdempotencyKey: queue,
		Payload:        map[string]string{"queue": queue},
	})
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}

	if err := NewPublisher(conn, nil).PublishCommand(ctx, envelope, routingKey); err != nil {
		t.Fatalf("publish command: %v", err)
	}

	delivery := waitForMessage(t, ctx, conn, consumer.dlqQueueName())
	if delivery.MessageId != envelope.MessageID {
		t.Fatalf("dlq message id = %q, want %q", delivery.MessageId, envelope.MessageID)
	}
	if attempt := retryAttemptFromHeaders(delivery.Headers); attempt != 4 {
		t.Fatalf("dlq retry attempt = %d, want 4", attempt)
	}
	if got := atomic.LoadInt32(&handled); got != 4 {
		t.Fatalf("handler calls = %d, want 4", got)
	}
}

func waitForMessage(t *testing.T, ctx context.Context, conn *Connection, queue string) amqp.Delivery {
	t.Helper()

	ch, err := conn.AMQP().Channel()
	if err != nil {
		t.Fatalf("open channel: %v", err)
	}
	defer ch.Close()

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		delivery, ok, err := ch.Get(queue, true)
		if err != nil {
			t.Fatalf("get message from %q: %v", queue, err)
		}
		if ok {
			return delivery
		}

		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for message in %q: %v", queue, ctx.Err())
		case <-ticker.C:
		}
	}
}

func cleanupQueues(t *testing.T, conn *Connection, queue string, levels []RetryLevel) {
	t.Helper()

	if conn == nil || conn.AMQP() == nil || conn.IsClosed() {
		return
	}
	ch, err := conn.AMQP().Channel()
	if err != nil {
		t.Logf("open cleanup channel: %v", err)
		return
	}
	defer ch.Close()

	names := []string{queue, queue + ".dlq"}
	for _, level := range levels {
		names = append(names, queue+".retry."+level.Name)
	}

	for _, name := range names {
		if _, err := ch.QueueDelete(name, false, false, false); err != nil {
			t.Logf("delete queue %q: %v", name, err)
		}
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
