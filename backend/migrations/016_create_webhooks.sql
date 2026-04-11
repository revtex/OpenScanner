-- +migrate Up
CREATE TABLE IF NOT EXISTS webhooks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    url          TEXT    NOT NULL,
    type         TEXT    NOT NULL DEFAULT 'generic',
    secret       TEXT,
    systems_json TEXT,
    disabled     INTEGER NOT NULL DEFAULT 0,
    "order"      INTEGER NOT NULL DEFAULT 0
);

-- +migrate Down
DROP TABLE IF EXISTS webhooks;
