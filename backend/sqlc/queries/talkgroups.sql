-- name: GetTalkgroup :one
SELECT * FROM talkgroups WHERE id = ? LIMIT 1;

-- name: GetTalkgroupBySystemAndTGID :one
SELECT * FROM talkgroups
WHERE system_id = ? AND talkgroup_id = ?
LIMIT 1;

-- name: ListTalkgroupsBySystem :many
SELECT * FROM talkgroups
WHERE system_id = ?
ORDER BY "order" ASC, talkgroup_id ASC;

-- name: ListAllTalkgroups :many
SELECT * FROM talkgroups ORDER BY system_id ASC, "order" ASC, talkgroup_id ASC;

-- name: CreateTalkgroup :one
INSERT INTO talkgroups (
    system_id,
    talkgroup_id,
    label,
    name,
    frequency,
    led,
    group_id,
    tag_id,
    "order"
) VALUES (
    :system_id,
    :talkgroup_id,
    :label,
    :name,
    :frequency,
    :led,
    :group_id,
    :tag_id,
    :order
) RETURNING id;

-- name: UpdateTalkgroup :exec
UPDATE talkgroups SET
    talkgroup_id = :talkgroup_id,
    label        = :label,
    name         = :name,
    frequency    = :frequency,
    led          = :led,
    group_id     = :group_id,
    tag_id       = :tag_id,
    "order"      = :order
WHERE id = :id;

-- name: DeleteTalkgroup :exec
DELETE FROM talkgroups WHERE id = ?;
