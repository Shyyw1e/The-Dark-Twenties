package rabbitmq

import (
	"context"
	"errors"
)

type HealthCheck struct {
	conn *Connection
}

func NewHealthCheck(conn *Connection) *HealthCheck {
	return &HealthCheck{conn: conn}
}

func (h *HealthCheck) Name() string {
	return "rabbitmq"
}

func (h *HealthCheck) Check(ctx context.Context) error {
	if h == nil || h.conn == nil {
		return errors.New("rabbitmq connection is nil")
	}
	if h.conn.IsClosed() {
		return errors.New("rabbitmq connection is closed")
	}
	if h.conn.AMQP() == nil {
		return errors.New("rabbitmq amqp connection is nil")
	}

	ch, err := h.conn.AMQP().Channel()
	if err != nil {
		return err
	}
	return ch.Close()
}
