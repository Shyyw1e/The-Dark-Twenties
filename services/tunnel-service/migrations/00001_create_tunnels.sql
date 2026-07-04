-- +goose Up
CREATE TABLE vpn_nodes (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    region TEXT NOT NULL,
    country TEXT NOT NULL,
    role TEXT NOT NULL,
    host TEXT NOT NULL,
    public_ip INET,
    status TEXT NOT NULL DEFAULT 'active',
    capacity INTEGER NOT NULL DEFAULT 0,
    load_score INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT vpn_nodes_role_check CHECK (role IN ('foreign_exit', 'ru_relay', 'bridge')),
    CONSTRAINT vpn_nodes_status_check CHECK (status IN ('active', 'degraded', 'draining', 'down', 'disabled')),
    CONSTRAINT vpn_nodes_capacity_check CHECK (capacity >= 0),
    CONSTRAINT vpn_nodes_load_score_check CHECK (load_score >= 0 AND load_score <= 100)
);

CREATE TABLE tunnels (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    device_id UUID NOT NULL,
    node_id UUID NOT NULL REFERENCES vpn_nodes (id),
    protocol TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    assigned_ip INET,
    public_key TEXT,
    private_key_encrypted BYTEA,
    external_ref TEXT,
    expires_at TIMESTAMPTZ,
    traffic_limit_bytes BIGINT,
    traffic_used_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tunnels_protocol_check CHECK (protocol IN ('wireguard', 'vless', 'vmess', 'hysteria2', 'tuic', 'shadowsocks')),
    CONSTRAINT tunnels_status_check CHECK (status IN ('active', 'disabled', 'expired', 'revoked', 'provisioning', 'failed')),
    CONSTRAINT tunnels_traffic_limit_bytes_check CHECK (traffic_limit_bytes IS NULL OR traffic_limit_bytes > 0),
    CONSTRAINT tunnels_traffic_used_bytes_check CHECK (traffic_used_bytes >= 0)
);

CREATE TABLE tunnel_configs (
    id UUID PRIMARY KEY,
    tunnel_id UUID NOT NULL REFERENCES tunnels (id) ON DELETE CASCADE,
    client_type TEXT NOT NULL,
    config_json_encrypted BYTEA,
    config_uri_encrypted BYTEA,
    qr_payload_encrypted BYTEA,
    version INTEGER NOT NULL DEFAULT 1,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tunnel_configs_version_check CHECK (version > 0)
);

CREATE TABLE traffic_usage (
    id UUID PRIMARY KEY,
    tunnel_id UUID NOT NULL REFERENCES tunnels (id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES vpn_nodes (id),
    rx_bytes BIGINT NOT NULL DEFAULT 0,
    tx_bytes BIGINT NOT NULL DEFAULT 0,
    collected_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT traffic_usage_rx_bytes_check CHECK (rx_bytes >= 0),
    CONSTRAINT traffic_usage_tx_bytes_check CHECK (tx_bytes >= 0)
);

CREATE INDEX vpn_nodes_region_idx ON vpn_nodes (region);
CREATE INDEX vpn_nodes_status_idx ON vpn_nodes (status);
CREATE INDEX tunnels_user_id_idx ON tunnels (user_id);
CREATE INDEX tunnels_device_id_idx ON tunnels (device_id);
CREATE INDEX tunnels_node_id_idx ON tunnels (node_id);
CREATE INDEX tunnels_status_idx ON tunnels (status);
CREATE INDEX tunnels_expires_at_idx ON tunnels (expires_at);
CREATE INDEX tunnel_configs_tunnel_id_idx ON tunnel_configs (tunnel_id);
CREATE INDEX traffic_usage_tunnel_collected_idx ON traffic_usage (tunnel_id, collected_at);

-- +goose Down
DROP INDEX IF EXISTS traffic_usage_tunnel_collected_idx;
DROP INDEX IF EXISTS tunnel_configs_tunnel_id_idx;
DROP INDEX IF EXISTS tunnels_expires_at_idx;
DROP INDEX IF EXISTS tunnels_status_idx;
DROP INDEX IF EXISTS tunnels_node_id_idx;
DROP INDEX IF EXISTS tunnels_device_id_idx;
DROP INDEX IF EXISTS tunnels_user_id_idx;
DROP INDEX IF EXISTS vpn_nodes_status_idx;
DROP INDEX IF EXISTS vpn_nodes_region_idx;
DROP TABLE IF EXISTS traffic_usage;
DROP TABLE IF EXISTS tunnel_configs;
DROP TABLE IF EXISTS tunnels;
DROP TABLE IF EXISTS vpn_nodes;
