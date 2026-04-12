-- +migrate Up
ALTER TABLE calls ADD COLUMN site TEXT;
ALTER TABLE calls ADD COLUMN channel TEXT;
ALTER TABLE calls ADD COLUMN decoder TEXT;

-- +migrate Down
-- SQLite does not support DROP COLUMN before 3.35. These columns will remain
-- but be unused if the migration is rolled back logically.
