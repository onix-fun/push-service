CREATE TABLE IF NOT EXISTS push_devices (
    id TEXT PRIMARY KEY,
    recipient_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    token TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, token)
);

CREATE INDEX IF NOT EXISTS push_devices_recipient_active ON push_devices(recipient_id, active);

CREATE TABLE IF NOT EXISTS push_messages (
    message_id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    attempts BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS push_deliveries (
    message_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    status TEXT NOT NULL,
    error TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(message_id, device_id)
);
