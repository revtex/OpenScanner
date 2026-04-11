-- name: GetAccess :one
SELECT * FROM accesses WHERE id = ? LIMIT 1;

-- name: GetAccessByCode :one
SELECT * FROM accesses WHERE code = ? LIMIT 1;

-- name: ListAccesses :many
SELECT * FROM accesses ORDER BY "order" ASC, id ASC;

-- name: CreateAccess :one
INSERT INTO accesses (
    code,
    ident,
    expiration,
    "limit",
    systems_json,
    "order"
) VALUES (
    :code,
    :ident,
    :expiration,
    :limit,
    :systems_json,
    :order
) RETURNING id;

-- name: UpdateAccess :exec
UPDATE accesses SET
    code         = :code,
    ident        = :ident,
    expiration   = :expiration,
    "limit"      = :limit,
    systems_json = :systems_json,
    "order"      = :order
WHERE id = :id;

-- name: DeleteAccess :exec
DELETE FROM accesses WHERE id = ?;
