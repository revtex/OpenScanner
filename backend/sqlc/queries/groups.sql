-- name: GetGroup :one
SELECT * FROM groups WHERE id = ? LIMIT 1;

-- name: ListGroups :many
SELECT * FROM groups ORDER BY label ASC;

-- name: CreateGroup :one
INSERT INTO groups (label) VALUES (:label) RETURNING id;

-- name: UpdateGroup :exec
UPDATE groups SET label = :label WHERE id = :id;

-- name: DeleteGroup :exec
DELETE FROM groups WHERE id = ?;
