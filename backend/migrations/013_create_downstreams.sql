-- +migrate Up
CREATE TABLE IF NOT EXISTS downstreams (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    url          TEXT    NOT NULL,
    api_key      TEXT    NOT NULL,
    systems_json TEXT,
    disabled     INTEGER NOT NULL DEFAULT 0,
    "order"      INTEGER NOT NULL DEFAULT 0
);

-- +migrate Down
DROP TABLE IF EXISTS downstreams;
