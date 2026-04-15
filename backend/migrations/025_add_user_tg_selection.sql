-- +migrate Up
ALTER TABLE users ADD COLUMN tg_selection_json TEXT;

-- +migrate Down
-- SQLite does not support DROP COLUMN directly; keep column on rollback.
SELECT 1;
