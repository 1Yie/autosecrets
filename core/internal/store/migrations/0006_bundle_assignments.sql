-- Secret Bundle Assignments (ADR-0018): an Assignment relates a Node Group
-- to a Secret Bundle (Application + Environment) and follows the Bundle's
-- current Desired Revision instead of pinning an obsolete snapshot.
ALTER TABLE assignments
  ADD COLUMN application_id uuid,
  ADD COLUMN environment_id uuid,
  ADD COLUMN status text NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'removing'));

UPDATE assignments a
SET application_id = br.application_id, environment_id = br.environment_id
FROM bundle_revisions br WHERE br.id = a.revision_id;

DELETE FROM assignments WHERE application_id IS NULL;

-- Legacy deployments could hold several Assignments per (group, bundle)
-- pinned to different revisions of the same Bundle. Keep only the newest
-- row per group + bundle so the unique index can be built.
DELETE FROM assignments a
USING assignments newer
WHERE newer.group_id = a.group_id
  AND newer.application_id = a.application_id
  AND newer.environment_id = a.environment_id
  AND (newer.created_at > a.created_at
       OR (newer.created_at = a.created_at AND newer.id > a.id));

ALTER TABLE assignments ALTER COLUMN application_id SET NOT NULL;
ALTER TABLE assignments ALTER COLUMN environment_id SET NOT NULL;
ALTER TABLE assignments DROP CONSTRAINT IF EXISTS assignments_group_id_revision_id_key;
CREATE UNIQUE INDEX assignments_group_bundle_uniq
  ON assignments (group_id, application_id, environment_id);
