-- +migrate Up
CREATE TABLE IF NOT EXISTS accesses (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    code         TEXT    NOT NULL,
    ident        TEXT,
    expiration   INTEGER,
    "limit"      INTEGER,
    systems_json TEXT,
    "order"      INTEGER NOT NULL DEFAULT 0
);

-- +migrate Down
DROP TABLE IF EXISTS accesses;
