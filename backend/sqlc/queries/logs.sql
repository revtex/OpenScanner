-- name: CreateLog :exec
INSERT INTO logs (date_time, level, message)
VALUES (:date_time, :level, :message);

-- name: ListLogs :many
SELECT * FROM logs ORDER BY date_time DESC;
