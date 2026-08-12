-- Environment protection classification (ADR-0016). Existing Environments
-- migrate to 'unclassified' and follow Protected rules until reviewed.
ALTER TABLE environments
  ADD COLUMN protection_level text NOT NULL DEFAULT 'unclassified'
    CHECK (protection_level IN ('standard', 'protected', 'unclassified'));
