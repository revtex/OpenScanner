-- +migrate Up
CREATE TABLE IF NOT EXISTS tags (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    label TEXT    UNIQUE NOT NULL
);

-- +migrate Down
DROP TABLE IF EXISTS tags;
