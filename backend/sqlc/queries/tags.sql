-- name: GetTag :one
SELECT * FROM tags WHERE id = ? LIMIT 1;

-- name: ListTags :many
SELECT * FROM tags ORDER BY label ASC;

-- name: CreateTag :one
INSERT INTO tags (label) VALUES (:label) RETURNING id;

-- name: UpdateTag :exec
UPDATE tags SET label = :label WHERE id = :id;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = ?;
