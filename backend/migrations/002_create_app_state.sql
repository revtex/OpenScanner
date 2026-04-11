-- +migrate Up
CREATE TABLE IF NOT EXISTS app_state (
    id             INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    setup_complete INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO app_state (id, setup_complete) VALUES (1, 0);

-- +migrate Down
DROP TABLE IF EXISTS app_state;
