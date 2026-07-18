package rabbitmq

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/message"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/processed"
	amqp "github.com/rabbitmq/amqp091-go"
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

type fakeProcessedStore struct {
	processed bool
	checkErr  error
	markErr   error
	marked    []processed.Record
}

func (s *fakeProcessedStore) IsProcessed(ctx context.Context, messageID, consumerName string) (bool, error) {
	if s.checkErr != nil {
		return false, s.checkErr
	}
	return s.processed, nil
}

func (s *fakeProcessedStore) MarkProcessed(ctx context.Context, record processed.Record) error {
	if s.markErr != nil {
		return s.markErr
	}
	s.marked = append(s.marked, record)
	return nil
}

func TestConsumerProcessEnvelopeMarksProcessedAfterSuccess(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	previousNow := timeNowUTC
	timeNowUTC = func() time.Time { return now }
	defer func() { timeNowUTC = previousNow }()

	store := &fakeProcessedStore{}
	var handled bool
	consumer := NewConsumer(nil, nil, ConsumerOptions{
		Name:              "subscription.payment-events",
		ProcessedMessages: store,
	}, HandlerFunc(func(ctx context.Context, envelope *message.Envelope) error {
		handled = true
		return nil
	}))
	envelope := testEnvelope(t)

	if err := consumer.processEnvelope(context.Background(), envelope); err != nil {
		t.Fatalf("processEnvelope returned error: %v", err)
	}

	if !handled {
		t.Fatal("handler was not called")
	}
	if len(store.marked) != 1 {
		t.Fatalf("marked count = %d, want 1", len(store.marked))
	}
	record := store.marked[0]
	if record.MessageID != envelope.MessageID || record.MessageType != envelope.MessageType || record.ConsumerName != "subscription.payment-events" {
		t.Fatalf("record = %+v", record)
	}
	if !record.ProcessedAt.Equal(now) || record.Status != processed.StatusProcessed {
		t.Fatalf("record time/status = %v/%q", record.ProcessedAt, record.Status)
	}
}

func TestConsumerProcessEnvelopeSkipsAlreadyProcessedMessage(t *testing.T) {
	store := &fakeProcessedStore{processed: true}
	var handled bool
	consumer := NewConsumer(nil, nil, ConsumerOptions{
		Name:              "subscription.payment-events",
		ProcessedMessages: store,
	}, HandlerFunc(func(ctx context.Context, envelope *message.Envelope) error {
		handled = true
		return nil
	}))

	if err := consumer.processEnvelope(context.Background(), testEnvelope(t)); err != nil {
		t.Fatalf("processEnvelope returned error: %v", err)
	}

	if handled {
		t.Fatal("handler must not be called for duplicate message")
	}
	if len(store.marked) != 0 {
		t.Fatalf("marked count = %d, want 0", len(store.marked))
	}
}

func TestConsumerProcessEnvelopeDoesNotMarkOnHandlerError(t *testing.T) {
	wantErr := errors.New("handler failed")
	store := &fakeProcessedStore{}
	consumer := NewConsumer(nil, nil, ConsumerOptions{
		Name:              "subscription.payment-events",
		ProcessedMessages: store,
	}, HandlerFunc(func(ctx context.Context, envelope *message.Envelope) error {
		return wantErr
	}))

	err := consumer.processEnvelope(context.Background(), testEnvelope(t))
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if len(store.marked) != 0 {
		t.Fatalf("marked count = %d, want 0", len(store.marked))
	}
}

func TestRetryAttemptFromHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers amqp.Table
		want    int
	}{
		{name: "nil", headers: nil, want: 0},
		{name: "missing", headers: amqp.Table{}, want: 0},
		{name: "int", headers: amqp.Table{retryAttemptHeader: 2}, want: 2},
		{name: "int32", headers: amqp.Table{retryAttemptHeader: int32(3)}, want: 3},
		{name: "string", headers: amqp.Table{retryAttemptHeader: "4"}, want: 4},
		{name: "invalid string", headers: amqp.Table{retryAttemptHeader: "oops"}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryAttemptFromHeaders(tt.headers); got != tt.want {
				t.Fatalf("retryAttemptFromHeaders() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConsumerNextRetryTargetUsesThreeRetryLevelsThenDLQ(t *testing.T) {
	consumer := NewConsumer(nil, nil, ConsumerOptions{
		Queue:      "subscription.payment-events",
		Exchange:   "vpn.events.topic",
		RoutingKey: "payment.succeeded",
		Retry: RetryPolicy{
			Enabled:     true,
			Exchange:    "vpn.retry",
			DLXExchange: "vpn.dlx",
		},
	}, HandlerFunc(func(ctx context.Context, envelope *message.Envelope) error {
		return nil
	}))

	first, err := consumer.nextRetryTarget(0)
	if err != nil {
		t.Fatalf("first target error: %v", err)
	}
	if first.deadLetter || first.attempt != 1 || first.routingKey != "subscription.payment-events.retry.1" || first.retryQueue != "subscription.payment-events.retry.10s" || first.retryDelay != 10*time.Second {
		t.Fatalf("first target = %+v", first)
	}

	second, err := consumer.nextRetryTarget(1)
	if err != nil {
		t.Fatalf("second target error: %v", err)
	}
	if second.deadLetter || second.attempt != 2 || second.routingKey != "subscription.payment-events.retry.2" || second.retryQueue != "subscription.payment-events.retry.1m" || second.retryDelay != time.Minute {
		t.Fatalf("second target = %+v", second)
	}

	third, err := consumer.nextRetryTarget(2)
	if err != nil {
		t.Fatalf("third target error: %v", err)
	}
	if third.deadLetter || third.attempt != 3 || third.routingKey != "subscription.payment-events.retry.3" || third.retryQueue != "subscription.payment-events.retry.5m" || third.retryDelay != 5*time.Minute {
		t.Fatalf("third target = %+v", third)
	}

	dlq, err := consumer.nextRetryTarget(3)
	if err != nil {
		t.Fatalf("dlq target error: %v", err)
	}
	if !dlq.deadLetter || dlq.attempt != 4 || dlq.exchange != "vpn.dlx" || dlq.routingKey != "subscription.payment-events.dlq" {
		t.Fatalf("dlq target = %+v", dlq)
	}
}

func TestConsumerValidateRetryPolicy(t *testing.T) {
	consumer := NewConsumer(nil, nil, ConsumerOptions{
		Queue:      "queue",
		Exchange:   "exchange",
		RoutingKey: "routing.key",
		Retry: RetryPolicy{
			Enabled: true,
		},
	}, HandlerFunc(func(ctx context.Context, envelope *message.Envelope) error {
		return nil
	}))

	err := consumer.validateRetryPolicy()
	if err == nil || !strings.Contains(err.Error(), "retry exchange") {
		t.Fatalf("error = %v, want retry exchange validation", err)
	}
}

func TestCopyHeaders(t *testing.T) {
	original := amqp.Table{"hello": "world"}
	copied := copyHeaders(original)
	copied["hello"] = "changed"

	if original["hello"] != "world" {
		t.Fatalf("original header changed: %+v", original)
	}
}

func testEnvelope(t *testing.T) *message.Envelope {
	t.Helper()

	envelope, err := message.NewEnvelope(message.NewEnvelopeParams{
		MessageType:    "payment.succeeded",
		Producer:       "billing-service",
		IdempotencyKey: "payment:1:succeeded",
		Payload:        map[string]string{"payment_id": "1"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope returned error: %v", err)
	}
	return envelope
}
