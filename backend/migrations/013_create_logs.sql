-- +migrate Up
CREATE TABLE IF NOT EXISTS logs (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    date_time INTEGER NOT NULL,
    level     TEXT    NOT NULL,
    message   TEXT    NOT NULL
);

-- +migrate Down
DROP TABLE IF EXISTS logs;
