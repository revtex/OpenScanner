-- +migrate Up
CREATE TABLE IF NOT EXISTS dirmonitors (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    directory    TEXT    NOT NULL,
    type         TEXT    NOT NULL,
    mask         TEXT,
    extension    TEXT,
    frequency    INTEGER,
    delay        INTEGER,
    delete_after INTEGER NOT NULL DEFAULT 0,
    use_polling  INTEGER NOT NULL DEFAULT 0,
    disabled     INTEGER NOT NULL DEFAULT 0,
    system_id    INTEGER REFERENCES systems(id),
    talkgroup_id INTEGER REFERENCES talkgroups(id),
    "order"      INTEGER NOT NULL DEFAULT 0
);

-- +migrate Down
DROP TABLE IF EXISTS dirmonitors;
