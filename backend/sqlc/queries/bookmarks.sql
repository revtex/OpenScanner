-- name: ListBookmarksByUser :many
SELECT * FROM bookmarks WHERE user_id = ? ORDER BY created_at DESC;

-- name: CreateBookmark :one
INSERT INTO bookmarks (call_id, user_id, session_id, created_at)
VALUES (:call_id, :user_id, :session_id, :created_at)
RETURNING id;

-- name: DeleteBookmarkByCallAndUser :exec
DELETE FROM bookmarks WHERE call_id = ? AND user_id = ?;

-- name: GetBookmarkByCallAndUser :one
SELECT * FROM bookmarks WHERE call_id = ? AND user_id = ? LIMIT 1;

-- name: ListBookmarkCallIDsByUser :many
SELECT call_id FROM bookmarks WHERE user_id = ? ORDER BY created_at DESC;

-- name: ListBookmarkCallsByUser :many
SELECT
    c.*,
    s.label AS system_label,
    s.system_id AS system_radio_id,
    t.label AS talkgroup_label,
    t.name  AS talkgroup_name,
    t.led   AS talkgroup_led,
    t.talkgroup_id AS talkgroup_radio_id,
    tr.text AS transcript_text
FROM bookmarks b
JOIN calls c ON c.id = b.call_id
LEFT JOIN systems s ON s.id = c.system_id
LEFT JOIN talkgroups t ON t.id = c.talkgroup_id
LEFT JOIN transcriptions tr ON tr.call_id = c.id
WHERE b.user_id = ?
ORDER BY b.created_at DESC
LIMIT 100;
