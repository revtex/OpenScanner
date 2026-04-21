-- +migrate Up
ALTER TABLE transcriptions ADD COLUMN segments TEXT;

-- +migrate Down
ALTER TABLE transcriptions DROP COLUMN segments;
