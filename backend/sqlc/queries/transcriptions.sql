-- name: GetTranscription :one
SELECT * FROM transcriptions WHERE id = ? LIMIT 1;

-- name: GetTranscriptionByCallID :one
SELECT * FROM transcriptions WHERE call_id = ? LIMIT 1;

-- name: CreateTranscription :one
INSERT INTO transcriptions (
    call_id,
    text,
    language,
    model,
    duration_ms,
    created_at
) VALUES (
    :call_id,
    :text,
    :language,
    :model,
    :duration_ms,
    :created_at
) RETURNING id;

-- name: DeleteTranscription :exec
DELETE FROM transcriptions WHERE id = ?;
