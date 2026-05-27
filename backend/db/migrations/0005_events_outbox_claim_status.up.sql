ALTER TABLE events_outbox
    ADD COLUMN status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN claimed_at TIMESTAMPTZ,
    ADD CONSTRAINT events_outbox_status_check
        CHECK (status IN ('pending', 'processing', 'delivered'));

UPDATE events_outbox
SET status = CASE WHEN delivered THEN 'delivered' ELSE 'pending' END,
    claimed_at = NULL;

DROP INDEX IF EXISTS events_outbox_dispatch_idx;

CREATE INDEX events_outbox_dispatch_idx
    ON events_outbox (status, available_at, claimed_at);
