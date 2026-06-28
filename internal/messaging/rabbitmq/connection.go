package rabbitmq

import (
	"context"
	"errors"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Connection struct {
	conn *amqp.Connection
	cfg  config.RabbitMQConfig
	log  logger.Logger
}

func Connect(ctx context.Context, cfg *config.RabbitMQConfig, log logger.Logger) (*Connection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return nil, errors.New("rabbitmq config is nil")
	}
	if log == nil {
		log = logger.FromContext(ctx)
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		log.Error("failed to connect rabbitmq", "error", err)
		return nil, err
	}

	rabbitConn := &Connection{
		conn: conn,
		cfg:  *cfg,
		log:  log,
	}

	if err := rabbitConn.DeclareTopology(); err != nil {
		_ = conn.Close()
		log.Error("failed to declare rabbitmq topology", "error", err)
		return nil, err
	}

	log.Info("connected to rabbitmq successfully")
	return rabbitConn, nil
}

func (c *Connection) AMQP() *amqp.Connection {
	if c == nil {
		return nil
	}
	return c.conn
}

func (c *Connection) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Connection) IsClosed() bool {
	if c == nil || c.conn == nil {
		return true
	}
	return c.conn.IsClosed()
}

func (c *Connection) DeclareTopology() error {
	if c == nil || c.conn == nil {
		return errors.New("rabbitmq connection is nil")
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := declareExchange(ch, c.cfg.EventsExchange, "topic"); err != nil {
		return err
	}
	if err := declareExchange(ch, c.cfg.CommandsExchange, "direct"); err != nil {
		return err
	}
	if err := declareExchange(ch, c.cfg.RetryExchange, "direct"); err != nil {
		return err
	}
	if err := declareExchange(ch, c.cfg.DLXExchange, "topic"); err != nil {
		return err
	}

	return nil
}

func declareExchange(ch *amqp.Channel, name, kind string) error {
	if name == "" {
		return errors.New("rabbitmq exchange name is empty")
	}

	return ch.ExchangeDeclare(
		name,
		kind,
		true,
		false,
		false,
		false,
		nil,
	)
}
