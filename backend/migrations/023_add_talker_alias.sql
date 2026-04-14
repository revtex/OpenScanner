-- +migrate Up
ALTER TABLE calls ADD COLUMN talker_alias TEXT;

-- +migrate Down
-- SQLite does not support DROP COLUMN before 3.35. The column will remain
-- but be unused if the migration is rolled back logically.
