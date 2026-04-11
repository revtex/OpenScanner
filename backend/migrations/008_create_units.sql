-- +migrate Up
CREATE TABLE IF NOT EXISTS units (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    system_id INTEGER NOT NULL REFERENCES systems(id) ON DELETE CASCADE,
    unit_id   INTEGER NOT NULL,
    label     TEXT,
    "order"   INTEGER NOT NULL DEFAULT 0
);

-- +migrate Down
DROP TABLE IF EXISTS units;
