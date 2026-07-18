-- +goose Up
CREATE TABLE processed_messages (
    message_id UUID NOT NULL,
    consumer_name TEXT NOT NULL,
    message_type TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    status TEXT NOT NULL DEFAULT 'processed',
    error TEXT,

    PRIMARY KEY (message_id, consumer_name),
    CONSTRAINT processed_messages_status_check CHECK (status IN ('processed'))
);

CREATE INDEX processed_messages_consumer_processed_at_idx
    ON processed_messages (consumer_name, processed_at DESC);

-- +goose Down
DROP INDEX IF EXISTS processed_messages_consumer_processed_at_idx;
DROP TABLE IF EXISTS processed_messages;
