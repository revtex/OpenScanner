-- +migrate Up
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id      INTEGER REFERENCES users(id) ON DELETE CASCADE,
    session_id   TEXT,
    endpoint     TEXT    NOT NULL,
    keys_json    TEXT    NOT NULL,
    systems_json TEXT,
    created_at   INTEGER NOT NULL
);

-- +migrate Down
DROP TABLE IF EXISTS push_subscriptions;
