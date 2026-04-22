-- name: CreateLog :exec
INSERT INTO logs (date_time, level, message)
VALUES (:date_time, :level, :message);
