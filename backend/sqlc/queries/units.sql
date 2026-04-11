-- name: GetUnit :one
SELECT * FROM units WHERE id = ? LIMIT 1;

-- name: GetUnitBySystemAndUnitID :one
SELECT * FROM units
WHERE system_id = ? AND unit_id = ?
LIMIT 1;

-- name: ListUnitsBySystem :many
SELECT * FROM units
WHERE system_id = ?
ORDER BY "order" ASC, unit_id ASC;

-- name: CreateUnit :one
INSERT INTO units (
    system_id,
    unit_id,
    label,
    "order"
) VALUES (
    :system_id,
    :unit_id,
    :label,
    :order
) RETURNING id;

-- name: UpdateUnit :exec
UPDATE units SET
    unit_id = :unit_id,
    label   = :label,
    "order" = :order
WHERE id = :id;

-- name: DeleteUnit :exec
DELETE FROM units WHERE id = ?;

-- name: UpsertUnit :exec
INSERT OR IGNORE INTO units (system_id, unit_id, label, "order")
VALUES (:system_id, :unit_id, :label, :order);

-- name: ListAllUnits :many
SELECT * FROM units ORDER BY system_id ASC, "order" ASC, unit_id ASC;

-- name: DeleteUnitsBySystem :exec
DELETE FROM units WHERE system_id = ?;
