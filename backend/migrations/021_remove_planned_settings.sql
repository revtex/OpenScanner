-- +migrate Up
-- Remove planned-but-unwired settings that were previously seeded as defaults.
-- These keys had no runtime consumers; future features will reintroduce them
-- with proper plans.
DELETE FROM settings WHERE key IN (
    'pushNotifications',
    'webhooksEnabled',
    'sortTalkgroups',
    'tagsToggle',
    'playbackGoesLive',
    'searchPatchedTalkgroups',
    'afsSystems'
);

-- +migrate Down
-- No-op: these rows were defaults with no semantic meaning. Re-seeding would
-- happen via the seed package on the next bootstrap if the keys were
-- reintroduced.
SELECT 1;
