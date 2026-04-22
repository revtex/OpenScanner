-- name: GetPushSubscription :one
SELECT * FROM push_subscriptions WHERE id = ? LIMIT 1;

-- name: ListAllActivePushSubscriptions :many
SELECT * FROM push_subscriptions ORDER BY created_at DESC;

-- name: CreatePushSubscription :one
INSERT INTO push_subscriptions (
    user_id,
    session_id,
    endpoint,
    keys_json,
    systems_json,
    created_at
) VALUES (
    :user_id,
    :session_id,
    :endpoint,
    :keys_json,
    :systems_json,
    :created_at
) RETURNING id;

-- name: DeletePushSubscription :exec
DELETE FROM push_subscriptions WHERE id = ?;

-- name: DeletePushSubscriptionByEndpoint :exec
DELETE FROM push_subscriptions WHERE endpoint = ?;
