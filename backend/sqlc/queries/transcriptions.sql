-- name: GetTranscription :one
SELECT * FROM transcriptions WHERE id = ? LIMIT 1;

-- name: GetTranscriptionByCallID :one
SELECT * FROM transcriptions WHERE call_id = ? LIMIT 1;

-- name: CreateTranscription :one
INSERT INTO transcriptions (
    call_id,
    text,
    segments,
    language,
    model,
    duration_ms,
    created_at
) VALUES (
    :call_id,
    :text,
    :segments,
    :language,
    :model,
    :duration_ms,
    :created_at
) RETURNING id;

-- name: SearchTranscriptions :many
SELECT t.id, t.call_id, t.text, t.segments, t.language, t.model,
       t.duration_ms, t.created_at,
       c.date_time, c.system_id, c.talkgroup_id
FROM transcriptions t
JOIN calls c ON c.id = t.call_id
WHERE t.text LIKE '%' || @query || '%'
ORDER BY c.date_time DESC
LIMIT @lim OFFSET @off;

-- name: CountTranscriptions :one
SELECT COUNT(*) FROM transcriptions;

-- name: DeleteTranscription :exec
DELETE FROM transcriptions WHERE id = ?;

-- name: DeleteTranscriptionByCallID :exec
DELETE FROM transcriptions WHERE call_id = ?;

-- name: TranscriptionStats :one
SELECT
    COUNT(*) AS total,
    COALESCE(AVG(duration_ms), 0) AS avg_duration_ms,
    COALESCE(MIN(duration_ms), 0) AS min_duration_ms,
    COALESCE(MAX(duration_ms), 0) AS max_duration_ms,
    COUNT(CASE WHEN created_at >= :since THEN 1 END) AS recent_count
FROM transcriptions;

-- name: TranscriptionsByLanguage :many
SELECT COALESCE(language, 'unknown') AS lang, COUNT(*) AS cnt
FROM transcriptions
GROUP BY language
ORDER BY cnt DESC
LIMIT 10;

-- name: TranscriptionsByModel :many
SELECT COALESCE(model, 'unknown') AS model_name, COUNT(*) AS cnt
FROM transcriptions
GROUP BY model
ORDER BY cnt DESC
LIMIT 10;
