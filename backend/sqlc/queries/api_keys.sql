-- name: GetAPIKey :one
SELECT * FROM api_keys WHERE id = ? LIMIT 1;

-- name: GetAPIKeyByKey :one
SELECT * FROM api_keys WHERE key = ? LIMIT 1;

-- name: ListAPIKeys :many
SELECT * FROM api_keys ORDER BY "order" ASC, id ASC;

-- name: CreateAPIKey :one
INSERT INTO api_keys (
    key,
    ident,
    disabled,
    systems_json,
    "order"
) VALUES (
    :key,
    :ident,
    :disabled,
    :systems_json,
    :order
) RETURNING id;

-- name: UpdateAPIKey :exec
UPDATE api_keys SET
    key          = :key,
    ident        = :ident,
    disabled     = :disabled,
    systems_json = :systems_json,
    "order"      = :order
WHERE id = :id;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys WHERE id = ?;
