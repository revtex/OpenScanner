-- +migrate Up
CREATE TABLE IF NOT EXISTS units (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    system_id INTEGER NOT NULL REFERENCES systems(id) ON DELETE CASCADE,
    unit_id   INTEGER NOT NULL,
    label     TEXT,
    "order"   INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_units_system_unit
    ON units (system_id, unit_id);

-- +migrate Down
DROP INDEX IF EXISTS idx_units_system_unit;
DROP TABLE IF EXISTS units;
