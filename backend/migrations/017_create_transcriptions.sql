-- +migrate Up
CREATE TABLE IF NOT EXISTS transcriptions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    call_id     INTEGER NOT NULL UNIQUE REFERENCES calls(id) ON DELETE CASCADE,
    text        TEXT    NOT NULL,
    segments    TEXT,
    language    TEXT,
    model       TEXT,
    duration_ms INTEGER,
    created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_transcriptions_text
    ON transcriptions(text);

-- +migrate Down
DROP INDEX IF EXISTS idx_transcriptions_text;
DROP TABLE IF EXISTS transcriptions;
