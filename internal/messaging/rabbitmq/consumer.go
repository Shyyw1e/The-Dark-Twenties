package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/message"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/messaging/processed"
	amqp "github.com/rabbitmq/amqp091-go"
)

const defaultPrefetchCount = 10

const (
	defaultRetry10s     = 10 * time.Second
	defaultRetry1m      = time.Minute
	defaultRetry5m      = 5 * time.Minute
	retryAttemptHeader  = "retry_attempt"
	retryErrorHeader    = "retry_error"
	retryConsumerHeader = "retry_consumer"
)

type MessageHandler interface {
	Handle(ctx context.Context, envelope *message.Envelope) error
}

type HandlerFunc func(ctx context.Context, envelope *message.Envelope) error

func (f HandlerFunc) Handle(ctx context.Context, envelope *message.Envelope) error {
	return f(ctx, envelope)
}

type RetryLevel struct {
	Name  string
	Delay time.Duration
}

type RetryPolicy struct {
	Enabled       bool
	MaxAttempts   int
	Exchange      string
	DLXExchange   string
	DLQRoutingKey string
	Levels        []RetryLevel
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
	ProcessedMessages    processed.Store
	Retry                RetryPolicy
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
	opts.Retry = normalizeRetryPolicy(conn, opts)

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
	if err := c.validateRetryPolicy(); err != nil {
		return err
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

	if c.opts.Retry.Enabled {
		if err := c.declareRetryTopology(ch); err != nil {
			return err
		}
	}

	return nil
}

func (c *Consumer) declareRetryTopology(ch *amqp.Channel) error {
	for index, level := range c.opts.Retry.Levels {
		queueName := c.retryQueueName(level)
		routingKey := c.retryRoutingKey(index)
		ttl := int64(level.Delay / time.Millisecond)

		if _, err := ch.QueueDeclare(
			queueName,
			true,
			false,
			false,
			false,
			amqp.Table{
				"x-message-ttl":             ttl,
				"x-dead-letter-exchange":    c.opts.Exchange,
				"x-dead-letter-routing-key": c.opts.RoutingKey,
			},
		); err != nil {
			return fmt.Errorf("declare rabbitmq retry queue %q: %w", queueName, err)
		}

		if err := ch.QueueBind(
			queueName,
			routingKey,
			c.opts.Retry.Exchange,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("bind rabbitmq retry queue %q: %w", queueName, err)
		}
	}

	if _, err := ch.QueueDeclare(
		c.dlqQueueName(),
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare rabbitmq dlq %q: %w", c.dlqQueueName(), err)
	}

	if err := ch.QueueBind(
		c.dlqQueueName(),
		c.dlqRoutingKey(),
		c.opts.Retry.DLXExchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind rabbitmq dlq %q: %w", c.dlqQueueName(), err)
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

	if err := c.processEnvelope(ctx, &envelope); err != nil {
		c.log.Error(
			"rabbitmq message processing failed",
			"consumer", c.Name(),
			"message_id", envelope.MessageID,
			"message_type", envelope.MessageType,
			"correlation_id", envelope.CorrelationID,
			"error", err,
		)

		if c.opts.Retry.Enabled {
			if retryErr := c.retryOrDeadLetter(ctx, delivery, &envelope, err); retryErr != nil {
				c.log.Error(
					"rabbitmq retry handling failed",
					"consumer", c.Name(),
					"message_id", envelope.MessageID,
					"message_type", envelope.MessageType,
					"correlation_id", envelope.CorrelationID,
					"error", retryErr,
				)
				c.nack(delivery, false)
			}
			return
		}

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

func (c *Consumer) processEnvelope(ctx context.Context, envelope *message.Envelope) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if envelope == nil {
		return errors.New("message envelope is nil")
	}

	consumerName := c.Name()
	if c.opts.ProcessedMessages != nil {
		alreadyProcessed, err := c.opts.ProcessedMessages.IsProcessed(ctx, envelope.MessageID, consumerName)
		if err != nil {
			return fmt.Errorf("check processed message: %w", err)
		}
		if alreadyProcessed {
			c.log.Info(
				"rabbitmq duplicate message skipped",
				"consumer", consumerName,
				"message_id", envelope.MessageID,
				"message_type", envelope.MessageType,
				"correlation_id", envelope.CorrelationID,
			)
			return nil
		}
	}

	if err := c.handler.Handle(ctx, envelope); err != nil {
		return err
	}

	if c.opts.ProcessedMessages != nil {
		if err := c.opts.ProcessedMessages.MarkProcessed(ctx, processed.Record{
			MessageID:    envelope.MessageID,
			MessageType:  envelope.MessageType,
			ConsumerName: consumerName,
			ProcessedAt:  timeNowUTC(),
			Status:       processed.StatusProcessed,
		}); err != nil {
			return fmt.Errorf("mark processed message: %w", err)
		}
	}

	return nil
}

func (c *Consumer) nack(delivery amqp.Delivery, requeue bool) {
	if err := delivery.Nack(false, requeue); err != nil {
		c.log.Error("failed to nack rabbitmq message", "consumer", c.Name(), "error", err)
	}
}

type retryTarget struct {
	exchange    string
	routingKey  string
	attempt     int
	deadLetter  bool
	retryDelay  time.Duration
	retryQueue  string
	description string
}

func (c *Consumer) retryOrDeadLetter(ctx context.Context, delivery amqp.Delivery, envelope *message.Envelope, cause error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if envelope == nil {
		return errors.New("message envelope is nil")
	}

	target, err := c.nextRetryTarget(retryAttemptFromHeaders(delivery.Headers))
	if err != nil {
		return err
	}

	headers := copyHeaders(delivery.Headers)
	headers[retryAttemptHeader] = int32(target.attempt)
	headers[retryConsumerHeader] = c.Name()
	if cause != nil {
		headers[retryErrorHeader] = cause.Error()
	}

	if err := c.publishEnvelope(ctx, target.exchange, target.routingKey, envelope, headers); err != nil {
		return err
	}

	if err := delivery.Ack(false); err != nil {
		return fmt.Errorf("ack original message after retry publish: %w", err)
	}

	fields := []any{
		"consumer", c.Name(),
		"message_id", envelope.MessageID,
		"message_type", envelope.MessageType,
		"correlation_id", envelope.CorrelationID,
		"attempt", target.attempt,
		"exchange", target.exchange,
		"routing_key", target.routingKey,
	}
	if target.deadLetter {
		c.log.Error("rabbitmq message moved to dlq", fields...)
		return nil
	}

	fields = append(fields, "retry_delay", target.retryDelay.String(), "retry_queue", target.retryQueue)
	c.log.Warn("rabbitmq message scheduled for retry", fields...)
	return nil
}

func (c *Consumer) publishEnvelope(ctx context.Context, exchange, routingKey string, envelope *message.Envelope, headers amqp.Table) error {
	if c == nil || c.conn == nil || c.conn.AMQP() == nil {
		return errors.New("rabbitmq consumer connection is nil")
	}
	if c.conn.IsClosed() {
		return errors.New("rabbitmq connection is closed")
	}
	if strings.TrimSpace(exchange) == "" {
		return errors.New("rabbitmq exchange is required")
	}
	if strings.TrimSpace(routingKey) == "" {
		return errors.New("rabbitmq routing key is required")
	}
	if envelope == nil {
		return errors.New("message envelope is nil")
	}
	if err := envelope.Validate(); err != nil {
		return fmt.Errorf("validate envelope: %w", err)
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	ch, err := c.conn.AMQP().Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}
	defer ch.Close()

	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("enable rabbitmq publisher confirms: %w", err)
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	if err := ch.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		true,
		false,
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			MessageId:     envelope.MessageID,
			CorrelationId: envelope.CorrelationID,
			Type:          envelope.MessageType,
			AppId:         envelope.Producer,
			Timestamp:     envelope.OccurredAt,
			Headers:       headers,
			Body:          body,
		},
	); err != nil {
		return fmt.Errorf("publish rabbitmq retry message: %w", err)
	}

	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			return errors.New("rabbitmq retry publish was not acknowledged")
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (c *Consumer) nextRetryTarget(currentAttempt int) (retryTarget, error) {
	if !c.opts.Retry.Enabled {
		return retryTarget{}, errors.New("rabbitmq retry policy is disabled")
	}
	nextAttempt := currentAttempt + 1
	if nextAttempt > c.opts.Retry.MaxAttempts {
		return retryTarget{
			exchange:    c.opts.Retry.DLXExchange,
			routingKey:  c.dlqRoutingKey(),
			attempt:     nextAttempt,
			deadLetter:  true,
			description: "dlq",
		}, nil
	}

	levelIndex := nextAttempt - 1
	if levelIndex >= len(c.opts.Retry.Levels) {
		levelIndex = len(c.opts.Retry.Levels) - 1
	}
	level := c.opts.Retry.Levels[levelIndex]

	return retryTarget{
		exchange:    c.opts.Retry.Exchange,
		routingKey:  c.retryRoutingKey(levelIndex),
		attempt:     nextAttempt,
		retryDelay:  level.Delay,
		retryQueue:  c.retryQueueName(level),
		description: level.Name,
	}, nil
}

func (c *Consumer) retryQueueName(level RetryLevel) string {
	return c.opts.Queue + ".retry." + level.Name
}

func (c *Consumer) retryRoutingKey(index int) string {
	return fmt.Sprintf("%s.retry.%d", c.opts.Queue, index+1)
}

func (c *Consumer) dlqQueueName() string {
	return c.opts.Queue + ".dlq"
}

func (c *Consumer) dlqRoutingKey() string {
	if strings.TrimSpace(c.opts.Retry.DLQRoutingKey) != "" {
		return c.opts.Retry.DLQRoutingKey
	}
	return c.opts.Queue + ".dlq"
}

func (c *Consumer) validateRetryPolicy() error {
	if !c.opts.Retry.Enabled {
		return nil
	}
	if strings.TrimSpace(c.opts.Retry.Exchange) == "" {
		return errors.New("rabbitmq retry exchange is required")
	}
	if strings.TrimSpace(c.opts.Retry.DLXExchange) == "" {
		return errors.New("rabbitmq retry dlx exchange is required")
	}
	if c.opts.Retry.MaxAttempts <= 0 {
		return errors.New("rabbitmq retry max attempts must be positive")
	}
	if len(c.opts.Retry.Levels) == 0 {
		return errors.New("rabbitmq retry levels are required")
	}
	for _, level := range c.opts.Retry.Levels {
		if strings.TrimSpace(level.Name) == "" {
			return errors.New("rabbitmq retry level name is required")
		}
		if level.Delay <= 0 {
			return errors.New("rabbitmq retry level delay must be positive")
		}
	}
	return nil
}

func normalizeRetryPolicy(conn *Connection, opts ConsumerOptions) RetryPolicy {
	retry := opts.Retry
	if !retry.Enabled {
		return retry
	}
	if retry.Exchange == "" && conn != nil {
		retry.Exchange = conn.cfg.RetryExchange
	}
	if retry.DLXExchange == "" && conn != nil {
		retry.DLXExchange = conn.cfg.DLXExchange
	}
	if len(retry.Levels) == 0 {
		retry.Levels = DefaultRetryLevels()
	}
	if retry.MaxAttempts <= 0 {
		retry.MaxAttempts = len(retry.Levels)
	}
	return retry
}

func DefaultRetryLevels() []RetryLevel {
	return []RetryLevel{
		{Name: "10s", Delay: defaultRetry10s},
		{Name: "1m", Delay: defaultRetry1m},
		{Name: "5m", Delay: defaultRetry5m},
	}
}

func retryAttemptFromHeaders(headers amqp.Table) int {
	if headers == nil {
		return 0
	}

	switch value := headers[retryAttemptHeader].(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	case uint:
		return int(value)
	case uint8:
		return int(value)
	case uint16:
		return int(value)
	case uint32:
		return int(value)
	case uint64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	case string:
		var attempt int
		if _, err := fmt.Sscanf(value, "%d", &attempt); err == nil {
			return attempt
		}
	}

	return 0
}

func copyHeaders(headers amqp.Table) amqp.Table {
	copied := amqp.Table{}
	for key, value := range headers {
		copied[key] = value
	}
	return copied
}

var timeNowUTC = func() time.Time {
	return time.Now().UTC()
}
