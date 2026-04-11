-- name: GetDirwatch :one
SELECT * FROM dirwatches WHERE id = ? LIMIT 1;

-- name: ListDirwatches :many
SELECT * FROM dirwatches ORDER BY "order" ASC, id ASC;

-- name: ListActiveDirwatches :many
SELECT * FROM dirwatches WHERE disabled = 0 ORDER BY "order" ASC, id ASC;

-- name: CreateDirwatch :one
INSERT INTO dirwatches (
    directory,
    type,
    mask,
    extension,
    frequency,
    delay,
    delete_after,
    use_polling,
    disabled,
    system_id,
    talkgroup_id,
    "order"
) VALUES (
    :directory,
    :type,
    :mask,
    :extension,
    :frequency,
    :delay,
    :delete_after,
    :use_polling,
    :disabled,
    :system_id,
    :talkgroup_id,
    :order
) RETURNING id;

-- name: UpdateDirwatch :exec
UPDATE dirwatches SET
    directory    = :directory,
    type         = :type,
    mask         = :mask,
    extension    = :extension,
    frequency    = :frequency,
    delay        = :delay,
    delete_after = :delete_after,
    use_polling  = :use_polling,
    disabled     = :disabled,
    system_id    = :system_id,
    talkgroup_id = :talkgroup_id,
    "order"      = :order
WHERE id = :id;

-- name: DeleteDirwatch :exec
DELETE FROM dirwatches WHERE id = ?;
