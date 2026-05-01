-- +migrate Up
CREATE TABLE IF NOT EXISTS tr_instances (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    label           TEXT    NOT NULL UNIQUE,
    instance_id     TEXT    NOT NULL,
    broker_url      TEXT    NOT NULL,
    base_topic      TEXT    NOT NULL,
    unit_topic      TEXT,
    message_topic   TEXT,
    username        TEXT,
    password_enc    TEXT,
    tls_skip_verify INTEGER NOT NULL DEFAULT 0,
    qos             INTEGER NOT NULL DEFAULT 0,
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    last_seen_at    INTEGER
);

CREATE INDEX IF NOT EXISTS idx_tr_instances_enabled ON tr_instances(enabled);

-- +migrate Down
DROP INDEX IF EXISTS idx_tr_instances_enabled;
DROP TABLE IF EXISTS tr_instances;
