-- +migrate Up
ALTER TABLE calls ADD COLUMN error_count INTEGER;
ALTER TABLE calls ADD COLUMN spike_count INTEGER;

-- +migrate Down
-- SQLite does not support DROP COLUMN before 3.35. These columns will remain
-- but be unused if the migration is rolled back logically.
