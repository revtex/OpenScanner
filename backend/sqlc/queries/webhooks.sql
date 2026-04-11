-- name: GetWebhook :one
SELECT * FROM webhooks WHERE id = ? LIMIT 1;

-- name: ListWebhooks :many
SELECT * FROM webhooks ORDER BY "order" ASC, id ASC;

-- name: ListActiveWebhooks :many
SELECT * FROM webhooks WHERE disabled = 0 ORDER BY "order" ASC, id ASC;

-- name: CreateWebhook :one
INSERT INTO webhooks (
    url,
    type,
    secret,
    systems_json,
    disabled,
    "order"
) VALUES (
    :url,
    :type,
    :secret,
    :systems_json,
    :disabled,
    :order
) RETURNING id;

-- name: UpdateWebhook :exec
UPDATE webhooks SET
    url          = :url,
    type         = :type,
    secret       = :secret,
    systems_json = :systems_json,
    disabled     = :disabled,
    "order"      = :order
WHERE id = :id;

-- name: DeleteWebhook :exec
DELETE FROM webhooks WHERE id = ?;
