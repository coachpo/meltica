-- name: EnqueueEvent :one
INSERT INTO events_outbox (
    aggregate_type,
    aggregate_id,
    event_type,
    payload,
    headers,
    available_at,
    status,
    claimed_at
)
VALUES (
    @aggregate_type::text,
    @aggregate_id::text,
    @event_type::text,
    COALESCE(@payload::jsonb, '{}'::jsonb),
    COALESCE(@headers::jsonb, '{}'::jsonb),
    COALESCE(@available_at::timestamptz, NOW()),
    'pending',
    NULL
)
RETURNING *;

-- name: ClaimPendingEvents :many
WITH claimable AS (
    SELECT id
    FROM events_outbox
    WHERE (
        status = 'pending'
        OR (status = 'processing' AND claimed_at < NOW() - sqlc.arg('lease')::interval)
    )
      AND available_at <= NOW()
    ORDER BY available_at ASC
    LIMIT sqlc.arg('limit')::int
    FOR UPDATE SKIP LOCKED
)
UPDATE events_outbox AS outbox
SET
    status = 'processing',
    claimed_at = NOW(),
    attempts = attempts + 1
FROM claimable
WHERE outbox.id = claimable.id
RETURNING outbox.*;

-- name: MarkEventDelivered :one
UPDATE events_outbox
SET
    status = 'delivered',
    delivered = TRUE,
    published_at = NOW(),
    claimed_at = NULL,
    last_error = NULL
WHERE id = @id::bigint
RETURNING *;

-- name: ResetEventForRetry :one
UPDATE events_outbox
SET
    status = 'pending',
    delivered = FALSE,
    claimed_at = NULL,
    last_error = @last_error::text,
    available_at = NOW() + INTERVAL '30 seconds'
WHERE id = @id::bigint
  AND status IN ('pending', 'processing')
RETURNING *;

-- name: DeleteEvent :exec
DELETE FROM events_outbox
WHERE id = @id::bigint;
