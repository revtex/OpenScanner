-- +migrate Up
CREATE UNIQUE INDEX IF NOT EXISTS idx_units_system_unit ON units (system_id, unit_id);

-- +migrate Down
DROP INDEX IF EXISTS idx_units_system_unit;
