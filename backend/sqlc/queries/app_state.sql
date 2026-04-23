-- name: GetAppState :one
SELECT * FROM app_state WHERE id = 1 LIMIT 1;

-- name: SetSetupComplete :exec
UPDATE app_state SET setup_complete = ? WHERE id = 1;
