-- name: GetBookmark :one
SELECT * FROM bookmarks WHERE id = ? LIMIT 1;

-- name: ListBookmarksByUser :many
SELECT * FROM bookmarks WHERE user_id = ? ORDER BY created_at DESC;

-- name: ListBookmarksBySession :many
SELECT * FROM bookmarks WHERE session_id = ? ORDER BY created_at DESC;

-- name: CreateBookmark :one
INSERT INTO bookmarks (call_id, user_id, session_id, created_at)
VALUES (:call_id, :user_id, :session_id, :created_at)
RETURNING id;

-- name: DeleteBookmark :exec
DELETE FROM bookmarks WHERE id = ?;

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
    t.label AS talkgroup_label,
    t.name  AS talkgroup_name,
    t.led   AS talkgroup_led
FROM bookmarks b
JOIN calls c ON c.id = b.call_id
LEFT JOIN systems s ON s.id = c.system_id
LEFT JOIN talkgroups t ON t.id = c.talkgroup_id
WHERE b.user_id = ?
ORDER BY b.created_at DESC
LIMIT 100;
