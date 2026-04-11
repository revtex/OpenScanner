-- +migrate Up
CREATE TABLE IF NOT EXISTS talkgroups (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    system_id    INTEGER NOT NULL REFERENCES systems(id) ON DELETE CASCADE,
    talkgroup_id INTEGER NOT NULL,
    label        TEXT,
    name         TEXT,
    frequency    INTEGER,
    led          TEXT,
    group_id     INTEGER REFERENCES groups(id),
    tag_id       INTEGER REFERENCES tags(id),
    "order"      INTEGER NOT NULL DEFAULT 0,
    UNIQUE (system_id, talkgroup_id)
);

-- +migrate Down
DROP TABLE IF EXISTS talkgroups;
