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
