-- name: GetCall :one
SELECT
    c.*,
    s.label  AS system_label,
    t.label  AS talkgroup_label,
    t.name   AS talkgroup_name
FROM calls c
LEFT JOIN systems    s ON s.id = c.system_id
LEFT JOIN talkgroups t ON t.id = c.talkgroup_id
WHERE c.id = ?
LIMIT 1;

-- name: ListCalls :many
SELECT c.*
FROM calls c
WHERE
    (sqlc.narg('system_id')    IS NULL OR c.system_id    = sqlc.narg('system_id'))
    AND (sqlc.narg('talkgroup_id') IS NULL OR c.talkgroup_id = sqlc.narg('talkgroup_id'))
    AND (sqlc.narg('date_from')    IS NULL OR c.date_time    >= sqlc.narg('date_from'))
    AND (sqlc.narg('date_to')      IS NULL OR c.date_time    <= sqlc.narg('date_to'))
ORDER BY c.date_time DESC
LIMIT  sqlc.narg('page_size')
OFFSET sqlc.narg('page_offset');

-- name: CreateCall :one
INSERT INTO calls (
    audio_path,
    audio_name,
    audio_type,
    date_time,
    frequency,
    duration,
    source,
    sources_json,
    frequencies_json,
    patches_json,
    system_id,
    talkgroup_id
) VALUES (
    :audio_path,
    :audio_name,
    :audio_type,
    :date_time,
    :frequency,
    :duration,
    :source,
    :sources_json,
    :frequencies_json,
    :patches_json,
    :system_id,
    :talkgroup_id
) RETURNING id;

-- name: DeleteCall :exec
DELETE FROM calls WHERE id = ?;

-- name: PruneCalls :exec
DELETE FROM calls WHERE date_time < ?;

-- name: CountCalls :one
SELECT COUNT(*) FROM calls;

-- name: GetLastCallForTalkgroup :one
SELECT c.date_time, c.duration
FROM calls c
WHERE c.system_id = ? AND c.talkgroup_id = ?
ORDER BY c.date_time DESC
LIMIT 1;

-- name: GetCallIDsOlderThan :many
SELECT id, audio_path
FROM calls
WHERE date_time < ?
ORDER BY date_time ASC
LIMIT 500;

-- name: DeleteCallBatch :exec
DELETE FROM calls WHERE id = ?;
