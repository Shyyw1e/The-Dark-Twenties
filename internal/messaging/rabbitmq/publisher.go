package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/message"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn *Connection
	log  logger.Logger
}

type PublishOptions struct {
	Exchange   string
	RoutingKey string
	Mandatory  bool
	Immediate  bool
}

func NewPublisher(conn *Connection, log logger.Logger) *Publisher {
	if log == nil {
		log = logger.FromContext(context.Background())
	}

	return &Publisher{
		conn: conn,
		log:  log,
	}
}

func (p *Publisher) PublishEvent(ctx context.Context, envelope *message.Envelope, routingKey string) error {
	if p == nil || p.conn == nil {
		return errors.New("rabbitmq publisher connection is nil")
	}

	return p.Publish(ctx, envelope, PublishOptions{
		Exchange:   p.conn.cfg.EventsExchange,
		RoutingKey: routingKey,
		Mandatory:  true,
	})
}

func (p *Publisher) PublishCommand(ctx context.Context, envelope *message.Envelope, routingKey string) error {
	if p == nil || p.conn == nil {
		return errors.New("rabbitmq publisher connection is nil")
	}

	return p.Publish(ctx, envelope, PublishOptions{
		Exchange:   p.conn.cfg.CommandsExchange,
		RoutingKey: routingKey,
		Mandatory:  true,
	})
}

func (p *Publisher) Publish(ctx context.Context, envelope *message.Envelope, opts PublishOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if p == nil || p.conn == nil || p.conn.AMQP() == nil {
		return errors.New("rabbitmq publisher connection is nil")
	}
	if p.conn.IsClosed() {
		return errors.New("rabbitmq connection is closed")
	}
	if envelope == nil {
		return errors.New("message envelope is nil")
	}
	if err := envelope.Validate(); err != nil {
		return fmt.Errorf("validate envelope: %w", err)
	}
	if strings.TrimSpace(opts.Exchange) == "" {
		return errors.New("rabbitmq exchange is required")
	}
	if strings.TrimSpace(opts.RoutingKey) == "" {
		return errors.New("rabbitmq routing key is required")
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	ch, err := p.conn.AMQP().Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}
	defer ch.Close()

	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("enable rabbitmq publisher confirms: %w", err)
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	err = ch.PublishWithContext(
		ctx,
		opts.Exchange,
		opts.RoutingKey,
		opts.Mandatory,
		opts.Immediate,
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			MessageId:     envelope.MessageID,
			CorrelationId: envelope.CorrelationID,
			Type:          envelope.MessageType,
			AppId:         envelope.Producer,
			Timestamp:     envelope.OccurredAt,
			Headers: amqp.Table{
				"message_version": envelope.MessageVersion,
				"causation_id":    envelope.CausationID,
				"idempotency_key": envelope.IdempotencyKey,
			},
			Body: body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish rabbitmq message: %w", err)
	}

	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			return errors.New("rabbitmq publish was not acknowledged")
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	p.log.Info(
		"rabbitmq message published",
		"exchange", opts.Exchange,
		"routing_key", opts.RoutingKey,
		"message_id", envelope.MessageID,
		"message_type", envelope.MessageType,
		"correlation_id", envelope.CorrelationID,
	)

	return nil
}
