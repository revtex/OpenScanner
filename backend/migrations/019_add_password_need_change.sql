-- +migrate Up
ALTER TABLE users ADD COLUMN password_need_change INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
ALTER TABLE users DROP COLUMN password_need_change;
