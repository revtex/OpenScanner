-- +migrate Up
UPDATE settings SET key = 'autoPopulateSystems' WHERE key = 'autoPopulate';
ALTER TABLE systems RENAME COLUMN auto_populate TO auto_populate_talkgroups;

-- +migrate Down
ALTER TABLE systems RENAME COLUMN auto_populate_talkgroups TO auto_populate;
UPDATE settings SET key = 'autoPopulate' WHERE key = 'autoPopulateSystems';
