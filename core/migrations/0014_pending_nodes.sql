-- Pending Managed Nodes exist before enrollment, and install tokens
-- can be bound to a reserved node id.
ALTER TABLE nodes ADD COLUMN bundle_dir text NOT NULL DEFAULT '';

ALTER TABLE enrollment_tokens
ADD COLUMN node_id uuid REFERENCES nodes (id) ON DELETE CASCADE;
