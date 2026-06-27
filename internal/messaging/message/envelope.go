package message

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Envelope struct {
	MessageID      string          `json:"message_id"`
	MessageType    string          `json:"message_type"`
	MessageVersion int             `json:"message_version"`
	OccurredAt     time.Time       `json:"occurred_at"`
	Producer       string          `json:"producer"`
	CorrelationID  string          `json:"correlation_id"`
	CausationID    string          `json:"causation_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key"`
	Payload        json.RawMessage `json:"payload"`
}

type NewEnvelopeParams struct {
	MessageType    string
	MessageVersion int
	Producer       string
	CorrelationID  string
	CausationID    string
	IdempotencyKey string
	Payload        any
}

func NewEnvelope(params NewEnvelopeParams) (*Envelope, error) {
	if params.MessageVersion == 0 {
		params.MessageVersion = 1
	}
	if strings.TrimSpace(params.CorrelationID) == "" {
		params.CorrelationID = uuid.NewString()
	}

	payload, err := json.Marshal(params.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	envelope := &Envelope{
		MessageID:      uuid.NewString(),
		MessageType:    params.MessageType,
		MessageVersion: params.MessageVersion,
		OccurredAt:     time.Now().UTC(),
		Producer:       params.Producer,
		CorrelationID:  params.CorrelationID,
		CausationID:    params.CausationID,
		IdempotencyKey: params.IdempotencyKey,
		Payload:        payload,
	}
	if err := envelope.Validate(); err != nil {
		return nil, err
	}

	return envelope, nil
}

func (e *Envelope) Validate() error {
	if e == nil {
		return errors.New("envelope is nil")
	}
	if strings.TrimSpace(e.MessageID) == "" {
		return errors.New("message_id is required")
	}
	if strings.TrimSpace(e.MessageType) == "" {
		return errors.New("message_type is required")
	}
	if e.MessageVersion <= 0 {
		return errors.New("message_version must be positive")
	}
	if e.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	if strings.TrimSpace(e.Producer) == "" {
		return errors.New("producer is required")
	}
	if strings.TrimSpace(e.CorrelationID) == "" {
		return errors.New("correlation_id is required")
	}
	if strings.TrimSpace(e.IdempotencyKey) == "" {
		return errors.New("idempotency_key is required")
	}
	if len(bytes.TrimSpace(e.Payload)) == 0 || bytes.Equal(bytes.TrimSpace(e.Payload), []byte("null")) {
		return errors.New("payload is required")
	}

	return nil
}
