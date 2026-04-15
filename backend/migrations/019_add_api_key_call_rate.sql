-- +migrate Up
ALTER TABLE api_keys ADD COLUMN call_rate_limit INTEGER;

-- +migrate Down
-- SQLite does not support DROP COLUMN directly; keep column on rollback.
SELECT 1;