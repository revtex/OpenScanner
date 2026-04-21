-- +migrate Up
ALTER TABLE shared_links ADD COLUMN expires_at INTEGER;

-- +migrate Down
ALTER TABLE shared_links DROP COLUMN expires_at;
