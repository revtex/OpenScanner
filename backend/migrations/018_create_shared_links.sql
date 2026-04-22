-- +migrate Up
CREATE TABLE IF NOT EXISTS shared_links (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    call_id    INTEGER NOT NULL UNIQUE REFERENCES calls(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT    UNIQUE NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_shared_links_token ON shared_links(token);

-- +migrate Down
DROP INDEX IF EXISTS idx_shared_links_token;
DROP TABLE IF EXISTS shared_links;
