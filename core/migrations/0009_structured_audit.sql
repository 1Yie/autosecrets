-- Structured Audit Events (ADR-0020): stable actor/action/resource/outcome
-- codes with event-time display snapshots and Operation Reason, so
-- investigation never parses free-form text.
ALTER TABLE audit_events
  ADD COLUMN actor_type text NOT NULL DEFAULT '',
  ADD COLUMN actor_id text NOT NULL DEFAULT '',
  ADD COLUMN actor_display text NOT NULL DEFAULT '',
  ADD COLUMN resource_type text NOT NULL DEFAULT '',
  ADD COLUMN resource_id text NOT NULL DEFAULT '',
  ADD COLUMN resource_display text NOT NULL DEFAULT '',
  ADD COLUMN outcome text NOT NULL DEFAULT '',
  ADD COLUMN operation_reason_category text NOT NULL DEFAULT '',
  ADD COLUMN operation_reason_explanation text NOT NULL DEFAULT '',
  ADD COLUMN operation_reason_external_ref text NOT NULL DEFAULT '';

UPDATE audit_events SET actor_display = actor, resource_display = resource, outcome = result;

CREATE INDEX idx_audit_actor_type ON audit_events (actor_type, actor_id);
CREATE INDEX idx_audit_resource_type ON audit_events (resource_type, resource_id);
CREATE INDEX idx_audit_outcome ON audit_events (outcome);
CREATE INDEX idx_audit_reason_category ON audit_events (operation_reason_category);
