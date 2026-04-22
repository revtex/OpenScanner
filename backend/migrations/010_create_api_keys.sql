-- +migrate Up
CREATE TABLE IF NOT EXISTS api_keys (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    key             TEXT    UNIQUE NOT NULL,
    ident           TEXT,
    disabled        INTEGER NOT NULL DEFAULT 0,
    systems_json    TEXT,
    call_rate_limit INTEGER,
    "order"         INTEGER NOT NULL DEFAULT 0
);

-- +migrate Down
DROP TABLE IF EXISTS api_keys;
