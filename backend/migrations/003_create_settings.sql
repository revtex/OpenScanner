-- +migrate Up
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- +migrate Down
DROP TABLE IF EXISTS settings;
