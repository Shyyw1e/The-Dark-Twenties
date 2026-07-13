package message

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewEnvelope(t *testing.T) {
	envelope, err := NewEnvelope(NewEnvelopeParams{
		MessageType:    "subscription.activated",
		Producer:       "subscription-service",
		IdempotencyKey: "subscription:1:activated",
		Payload:        map[string]string{"subscription_id": "1"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope returned error: %v", err)
	}

	if envelope.MessageID == "" || envelope.CorrelationID == "" {
		t.Fatalf("ids are empty: %+v", envelope)
	}
	if envelope.MessageVersion != 1 {
		t.Fatalf("message version = %d, want 1", envelope.MessageVersion)
	}
	if envelope.OccurredAt.IsZero() {
		t.Fatal("occurred_at is zero")
	}

	var payload map[string]string
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["subscription_id"] != "1" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestEnvelopeValidate(t *testing.T) {
	envelope, err := NewEnvelope(NewEnvelopeParams{
		MessageType:    "subscription.activated",
		Producer:       "subscription-service",
		IdempotencyKey: "subscription:1:activated",
		Payload:        map[string]string{"subscription_id": "1"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope returned error: %v", err)
	}

	envelope.MessageType = ""
	err = envelope.Validate()
	if err == nil || !strings.Contains(err.Error(), "message_type") {
		t.Fatalf("error = %v, want message_type validation", err)
	}
}

func TestNewEnvelopeRejectsMissingPayload(t *testing.T) {
	_, err := NewEnvelope(NewEnvelopeParams{
		MessageType:    "subscription.activated",
		Producer:       "subscription-service",
		IdempotencyKey: "subscription:1:activated",
		Payload:        nil,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
