-- +migrate Up
CREATE TABLE IF NOT EXISTS calls (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    audio_path       TEXT    NOT NULL,
    audio_name       TEXT    NOT NULL,
    audio_type       TEXT    NOT NULL,
    date_time        INTEGER NOT NULL,
    frequency        INTEGER,
    duration         INTEGER,
    source           INTEGER,
    sources_json     TEXT,
    frequencies_json TEXT,
    patches_json     TEXT,
    system_id        INTEGER NOT NULL REFERENCES systems(id) ON DELETE CASCADE,
    talkgroup_id     INTEGER REFERENCES talkgroups(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_calls_datetime_system_tg
    ON calls(date_time, system_id, talkgroup_id);

CREATE INDEX IF NOT EXISTS idx_calls_system_tg
    ON calls(system_id, talkgroup_id);

-- +migrate Down
DROP INDEX IF EXISTS idx_calls_system_tg;
DROP INDEX IF EXISTS idx_calls_datetime_system_tg;
DROP TABLE IF EXISTS calls;
