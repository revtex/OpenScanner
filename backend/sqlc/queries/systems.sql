-- name: GetSystem :one
SELECT * FROM systems WHERE id = ? LIMIT 1;

-- name: GetSystemBySystemID :one
SELECT * FROM systems WHERE system_id = ? LIMIT 1;

-- name: GetSystemByLabel :one
SELECT * FROM systems WHERE label = ? LIMIT 1;

-- name: ListSystems :many
SELECT * FROM systems ORDER BY "order" ASC, id ASC;

-- name: CreateSystem :one
INSERT INTO systems (
    system_id,
    label,
    auto_populate,
    blacklists_json,
    led,
    "order"
) VALUES (
    :system_id,
    :label,
    :auto_populate,
    :blacklists_json,
    :led,
    :order
) RETURNING id;

-- name: UpdateSystem :exec
UPDATE systems SET
    system_id       = :system_id,
    label           = :label,
    auto_populate   = :auto_populate,
    blacklists_json = :blacklists_json,
    led             = :led,
    "order"         = :order
WHERE id = :id;

-- name: DeleteSystem :exec
DELETE FROM systems WHERE id = ?;
