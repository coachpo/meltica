DROP INDEX IF EXISTS events_outbox_dispatch_idx;

ALTER TABLE events_outbox
    DROP CONSTRAINT IF EXISTS events_outbox_status_check,
    DROP COLUMN IF EXISTS claimed_at,
    DROP COLUMN IF EXISTS status;

CREATE INDEX events_outbox_dispatch_idx
    ON events_outbox (delivered, available_at);
