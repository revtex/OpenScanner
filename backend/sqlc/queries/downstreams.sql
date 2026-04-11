-- name: GetDownstream :one
SELECT * FROM downstreams WHERE id = ? LIMIT 1;

-- name: ListDownstreams :many
SELECT * FROM downstreams ORDER BY "order" ASC, id ASC;

-- name: ListActiveDownstreams :many
SELECT * FROM downstreams WHERE disabled = 0 ORDER BY "order" ASC, id ASC;

-- name: CreateDownstream :one
INSERT INTO downstreams (
    url,
    api_key,
    systems_json,
    disabled,
    "order"
) VALUES (
    :url,
    :api_key,
    :systems_json,
    :disabled,
    :order
) RETURNING id;

-- name: UpdateDownstream :exec
UPDATE downstreams SET
    url          = :url,
    api_key      = :api_key,
    systems_json = :systems_json,
    disabled     = :disabled,
    "order"      = :order
WHERE id = :id;

-- name: DeleteDownstream :exec
DELETE FROM downstreams WHERE id = ?;
