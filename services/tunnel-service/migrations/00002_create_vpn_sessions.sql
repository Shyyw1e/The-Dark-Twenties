-- +goose Up
CREATE TABLE vpn_sessions (
    id UUID PRIMARY KEY,
    tunnel_id UUID NOT NULL REFERENCES tunnels (id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES vpn_nodes (id),
    protocol TEXT NOT NULL,
    source_ip_hash TEXT,
    user_agent TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at TIMESTAMPTZ,
    rx_bytes BIGINT NOT NULL DEFAULT 0,
    tx_bytes BIGINT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT vpn_sessions_protocol_check CHECK (protocol IN ('wireguard', 'vless', 'vmess', 'hysteria2', 'tuic', 'shadowsocks')),
    CONSTRAINT vpn_sessions_status_check CHECK (status IN ('active', 'closed', 'expired', 'unknown')),
    CONSTRAINT vpn_sessions_rx_bytes_check CHECK (rx_bytes >= 0),
    CONSTRAINT vpn_sessions_tx_bytes_check CHECK (tx_bytes >= 0),
    CONSTRAINT vpn_sessions_seen_period_check CHECK (last_seen_at >= started_at)
);

CREATE INDEX vpn_sessions_tunnel_id_idx ON vpn_sessions (tunnel_id);
CREATE INDEX vpn_sessions_node_id_idx ON vpn_sessions (node_id);
CREATE INDEX vpn_sessions_status_idx ON vpn_sessions (status);
CREATE INDEX vpn_sessions_last_seen_at_idx ON vpn_sessions (last_seen_at);

-- +goose Down
DROP INDEX IF EXISTS vpn_sessions_last_seen_at_idx;
DROP INDEX IF EXISTS vpn_sessions_status_idx;
DROP INDEX IF EXISTS vpn_sessions_node_id_idx;
DROP INDEX IF EXISTS vpn_sessions_tunnel_id_idx;
DROP TABLE IF EXISTS vpn_sessions;
