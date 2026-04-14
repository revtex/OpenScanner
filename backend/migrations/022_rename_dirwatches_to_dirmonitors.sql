-- +migrate Up
ALTER TABLE dirwatches RENAME TO dirmonitors;

-- +migrate Down
ALTER TABLE dirmonitors RENAME TO dirwatches;
