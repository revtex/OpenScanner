-- +migrate Up
CREATE TABLE IF NOT EXISTS users (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    username             TEXT    UNIQUE NOT NULL,
    password_hash        TEXT    NOT NULL,
    role                 TEXT    NOT NULL DEFAULT 'listener',
    disabled             INTEGER NOT NULL DEFAULT 0,
    systems_json         TEXT,
    expiration           INTEGER,
    "limit"              INTEGER,
    password_need_change INTEGER NOT NULL DEFAULT 0,
    tg_selection_json    TEXT,
    created_at           INTEGER NOT NULL,
    updated_at           INTEGER NOT NULL
);

-- +migrate Down
DROP TABLE IF EXISTS users;
-- +migrate Up
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    UNIQUE NOT NULL,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'listener',
    disabled      INTEGER NOT NULL DEFAULT 0,
    systems_json  TEXT,
    expiration    INTEGER,
    "limit"       INTEGER,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);

-- +migrate Down
DROP TABLE IF EXISTS users;
