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

-- name: CountTranscriptions :one
SELECT COUNT(*) FROM transcriptions;

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
