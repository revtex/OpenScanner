-- name: GetTRInstance :one
SELECT * FROM tr_instances WHERE id = ? LIMIT 1;

-- name: GetTRInstanceByLabel :one
SELECT * FROM tr_instances WHERE label = ? LIMIT 1;

-- name: ListTRInstances :many
SELECT * FROM tr_instances ORDER BY label ASC;

-- name: ListEnabledTRInstances :many
SELECT * FROM tr_instances WHERE enabled = 1 ORDER BY label ASC;

-- name: CreateTRInstance :one
INSERT INTO tr_instances (
    label,
    instance_id,
    broker_url,
    base_topic,
    unit_topic,
    message_topic,
    username,
    password_enc,
    tls_skip_verify,
    qos,
    enabled,
    created_at,
    updated_at
) VALUES (
    :label,
    :instance_id,
    :broker_url,
    :base_topic,
    :unit_topic,
    :message_topic,
    :username,
    :password_enc,
    :tls_skip_verify,
    :qos,
    :enabled,
    :created_at,
    :updated_at
) RETURNING *;

-- name: UpdateTRInstance :one
UPDATE tr_instances SET
    label           = :label,
    instance_id     = :instance_id,
    broker_url      = :broker_url,
    base_topic      = :base_topic,
    unit_topic      = :unit_topic,
    message_topic   = :message_topic,
    username        = :username,
    password_enc    = :password_enc,
    tls_skip_verify = :tls_skip_verify,
    qos             = :qos,
    enabled         = :enabled,
    updated_at      = :updated_at
WHERE id = :id
RETURNING *;

-- name: UpdateTRInstancePassword :exec
UPDATE tr_instances SET
    password_enc = :password_enc,
    updated_at   = :updated_at
WHERE id = :id;

-- name: DeleteTRInstance :exec
DELETE FROM tr_instances WHERE id = ?;

-- name: TouchTRInstanceLastSeen :exec
UPDATE tr_instances SET last_seen_at = :last_seen_at WHERE id = :id;
