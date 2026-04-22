-- name: GetSetting :one
SELECT * FROM settings WHERE key = ? LIMIT 1;

-- name: ListSettings :many
SELECT * FROM settings ORDER BY key ASC;

-- name: UpsertSetting :exec
INSERT OR REPLACE INTO settings (key, value) VALUES (:key, :value);
