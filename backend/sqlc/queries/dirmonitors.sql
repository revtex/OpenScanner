-- name: GetDirMonitor :one
SELECT * FROM dirmonitors WHERE id = ? LIMIT 1;

-- name: ListDirMonitors :many
SELECT * FROM dirmonitors ORDER BY "order" ASC, id ASC;

-- name: ListActiveDirMonitors :many
SELECT * FROM dirmonitors WHERE disabled = 0 ORDER BY "order" ASC, id ASC;

-- name: CreateDirMonitor :one
INSERT INTO dirmonitors (
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

-- name: UpdateDirMonitor :exec
UPDATE dirmonitors SET
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

-- name: DeleteDirMonitor :exec
DELETE FROM dirmonitors WHERE id = ?;
