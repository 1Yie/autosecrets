-- Operation Reason on every Bundle Revision (ADR-0020): Desired State
-- changes stay attributable with a stable category, an explanation, and an
-- optional external change/incident reference.
ALTER TABLE bundle_revisions
  ADD COLUMN operation_reason_category text NOT NULL DEFAULT '',
  ADD COLUMN operation_reason_explanation text NOT NULL DEFAULT '',
  ADD COLUMN operation_reason_external_ref text NOT NULL DEFAULT '';
