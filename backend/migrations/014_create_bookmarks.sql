-- +migrate Up
CREATE TABLE IF NOT EXISTS bookmarks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    call_id    INTEGER NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    user_id    INTEGER REFERENCES users(id) ON DELETE CASCADE,
    session_id TEXT,
    created_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bookmarks_user_call
    ON bookmarks(user_id, call_id) WHERE user_id IS NOT NULL;

-- +migrate Down
DROP INDEX IF EXISTS idx_bookmarks_user_call;
DROP TABLE IF EXISTS bookmarks;
