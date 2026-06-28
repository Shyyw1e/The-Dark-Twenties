package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/message"
	amqp "github.com/rabbitmq/amqp091-go"
)

const defaultPrefetchCount = 10

type MessageHandler interface {
	Handle(ctx context.Context, envelope *message.Envelope) error
}

type HandlerFunc func(ctx context.Context, envelope *message.Envelope) error

func (f HandlerFunc) Handle(ctx context.Context, envelope *message.Envelope) error {
	return f(ctx, envelope)
}

type ConsumerOptions struct {
	Name                 string
	Queue                string
	Exchange             string
	RoutingKey           string
	ConsumerTag          string
	PrefetchCount        int
	RequeueOnError       bool
	DeadLetterExchange   string
	DeadLetterRoutingKey string
}

type Consumer struct {
	conn    *Connection
	log     logger.Logger
	opts    ConsumerOptions
	handler MessageHandler

	mu     sync.Mutex
	ch     *amqp.Channel
	cancel context.CancelFunc
	done   chan struct{}
}

func NewConsumer(conn *Connection, log logger.Logger, opts ConsumerOptions, handler MessageHandler) *Consumer {
	if log == nil {
		log = logger.FromContext(context.Background())
	}
	if opts.PrefetchCount <= 0 {
		opts.PrefetchCount = defaultPrefetchCount
	}
	if opts.ConsumerTag == "" {
		opts.ConsumerTag = opts.Queue
	}
	if opts.Name == "" {
		opts.Name = "rabbitmq-consumer"
	}

	return &Consumer{
		conn:    conn,
		log:     log,
		opts:    opts,
		handler: handler,
		done:    make(chan struct{}),
	}
}

func NewEventConsumer(conn *Connection, log logger.Logger, opts ConsumerOptions, handler MessageHandler) *Consumer {
	if conn != nil && opts.Exchange == "" {
		opts.Exchange = conn.cfg.EventsExchange
	}
	return NewConsumer(conn, log, opts, handler)
}

func NewCommandConsumer(conn *Connection, log logger.Logger, opts ConsumerOptions, handler MessageHandler) *Consumer {
	if conn != nil && opts.Exchange == "" {
		opts.Exchange = conn.cfg.CommandsExchange
	}
	return NewConsumer(conn, log, opts, handler)
}

func (c *Consumer) Name() string {
	if c == nil || c.opts.Name == "" {
		return "rabbitmq-consumer"
	}
	return c.opts.Name
}

func (c *Consumer) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.validate(); err != nil {
		return err
	}
	if c.conn.IsClosed() {
		return errors.New("rabbitmq connection is closed")
	}

	ch, err := c.conn.AMQP().Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}

	if err := ch.Qos(c.opts.PrefetchCount, 0, false); err != nil {
		_ = ch.Close()
		return fmt.Errorf("set rabbitmq qos: %w", err)
	}

	if err := c.declareQueue(ch); err != nil {
		_ = ch.Close()
		return err
	}

	deliveries, err := ch.Consume(
		c.opts.Queue,
		c.opts.ConsumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("start rabbitmq consume: %w", err)
	}

	consumerCtx, cancel := context.WithCancel(ctx)

	c.mu.Lock()
	c.ch = ch
	c.cancel = cancel
	c.done = make(chan struct{})
	c.mu.Unlock()

	go c.consumeLoop(consumerCtx, deliveries)

	c.log.Info(
		"rabbitmq consumer started",
		"consumer", c.Name(),
		"queue", c.opts.Queue,
		"exchange", c.opts.Exchange,
		"routing_key", c.opts.RoutingKey,
	)

	return nil
}

func (c *Consumer) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	cancel := c.cancel
	ch := c.ch
	done := c.done
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if ch != nil {
		_ = ch.Cancel(c.opts.ConsumerTag, false)
		_ = ch.Close()
	}

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	c.log.Info("rabbitmq consumer stopped", "consumer", c.Name(), "queue", c.opts.Queue)
	return nil
}

func (c *Consumer) validate() error {
	if c == nil {
		return errors.New("rabbitmq consumer is nil")
	}
	if c.conn == nil || c.conn.AMQP() == nil {
		return errors.New("rabbitmq consumer connection is nil")
	}
	if c.handler == nil {
		return errors.New("rabbitmq consumer handler is nil")
	}
	if strings.TrimSpace(c.opts.Queue) == "" {
		return errors.New("rabbitmq consumer queue is required")
	}
	if strings.TrimSpace(c.opts.Exchange) == "" {
		return errors.New("rabbitmq consumer exchange is required")
	}
	if strings.TrimSpace(c.opts.RoutingKey) == "" {
		return errors.New("rabbitmq consumer routing key is required")
	}
	return nil
}

func (c *Consumer) declareQueue(ch *amqp.Channel) error {
	args := amqp.Table{}
	if c.opts.DeadLetterExchange != "" {
		args["x-dead-letter-exchange"] = c.opts.DeadLetterExchange
	}
	if c.opts.DeadLetterRoutingKey != "" {
		args["x-dead-letter-routing-key"] = c.opts.DeadLetterRoutingKey
	}

	if _, err := ch.QueueDeclare(
		c.opts.Queue,
		true,
		false,
		false,
		false,
		args,
	); err != nil {
		return fmt.Errorf("declare rabbitmq queue: %w", err)
	}

	if err := ch.QueueBind(
		c.opts.Queue,
		c.opts.RoutingKey,
		c.opts.Exchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind rabbitmq queue: %w", err)
	}

	return nil
}

func (c *Consumer) consumeLoop(ctx context.Context, deliveries <-chan amqp.Delivery) {
	defer close(c.done)

	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			c.handleDelivery(ctx, delivery)
		}
	}
}

func (c *Consumer) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	var envelope message.Envelope
	if err := json.Unmarshal(delivery.Body, &envelope); err != nil {
		c.log.Error("failed to decode rabbitmq message", "consumer", c.Name(), "error", err)
		c.nack(delivery, false)
		return
	}

	if err := envelope.Validate(); err != nil {
		c.log.Error(
			"invalid rabbitmq message envelope",
			"consumer", c.Name(),
			"message_id", envelope.MessageID,
			"message_type", envelope.MessageType,
			"error", err,
		)
		c.nack(delivery, false)
		return
	}

	if err := c.handler.Handle(ctx, &envelope); err != nil {
		c.log.Error(
			"rabbitmq message handler failed",
			"consumer", c.Name(),
			"message_id", envelope.MessageID,
			"message_type", envelope.MessageType,
			"correlation_id", envelope.CorrelationID,
			"error", err,
		)
		c.nack(delivery, c.opts.RequeueOnError)
		return
	}

	if err := delivery.Ack(false); err != nil {
		c.log.Error(
			"failed to ack rabbitmq message",
			"consumer", c.Name(),
			"message_id", envelope.MessageID,
			"message_type", envelope.MessageType,
			"error", err,
		)
		return
	}

	c.log.Info(
		"rabbitmq message consumed",
		"consumer", c.Name(),
		"message_id", envelope.MessageID,
		"message_type", envelope.MessageType,
		"correlation_id", envelope.CorrelationID,
	)
}

func (c *Consumer) nack(delivery amqp.Delivery, requeue bool) {
	if err := delivery.Nack(false, requeue); err != nil {
		c.log.Error("failed to nack rabbitmq message", "consumer", c.Name(), "error", err)
	}
}
