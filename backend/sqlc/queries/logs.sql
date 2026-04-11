-- name: CreateLog :exec
INSERT INTO logs (date_time, level, message)
VALUES (:date_time, :level, :message);

-- name: ListLogs :many
SELECT * FROM logs ORDER BY date_time DESC;

-- name: PruneLogs :exec
DELETE FROM logs WHERE date_time < ?;

-- name: ListLogsByDateRange :many
SELECT * FROM logs
WHERE date_time >= ? AND date_time <= ?
ORDER BY date_time DESC;

-- name: ListLogsByDateRangeAndLevel :many
SELECT * FROM logs
WHERE date_time >= ? AND date_time <= ? AND level = ?
ORDER BY date_time DESC;
