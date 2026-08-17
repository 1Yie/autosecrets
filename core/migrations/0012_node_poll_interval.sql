-- Per-node Agent polling interval, adjustable through the management API
-- and delivered to the Agent in the desired/heartbeat responses.

ALTER TABLE nodes
ADD COLUMN poll_interval_seconds integer NOT NULL DEFAULT 15
CHECK (poll_interval_seconds BETWEEN 5 AND 86400);
