-- name: ListApplications :many
SELECT id, name, created_at FROM applications ORDER BY name;

-- name: CreateApplication :exec
INSERT INTO applications (id, name) VALUES ($1, $2);

-- name: GetApplication :one
SELECT id, name, created_at FROM applications WHERE id = $1;

-- name: ListEnvironments :many
SELECT id, name, application_id, protection_level FROM environments
WHERE application_id = $1 ORDER BY name;

-- name: CreateEnvironment :exec
INSERT INTO environments (id, application_id, name, protection_level) VALUES ($1, $2, $3, $4);

-- name: GetEnvironment :one
SELECT id, name, application_id, protection_level FROM environments
WHERE id = $1 AND application_id = $2;

-- name: ListSecrets :many
SELECT sc.id, sc.name,
       COALESCE(fb.path, ''), COALESCE(fb.uid, 0), COALESCE(fb.gid, 0), COALESCE(fb.mode, 0),
       COALESCE(latest.seq, 0)::bigint, COALESCE(ds.version_seq, 0)
FROM secrets sc
LEFT JOIN file_bindings fb ON fb.secret_id = sc.id
LEFT JOIN LATERAL (SELECT max(seq) AS seq FROM secret_versions sv WHERE sv.secret_id = sc.id) latest ON true
LEFT JOIN drafts d ON d.application_id = sc.application_id AND d.environment_id = sc.environment_id
LEFT JOIN draft_selections ds ON ds.draft_id = d.id AND ds.secret_id = sc.id
WHERE sc.application_id = $1 AND sc.environment_id = $2 AND sc.retired_at IS NULL
ORDER BY sc.name;

-- name: InsertSecret :exec
INSERT INTO secrets (id, application_id, environment_id, name) VALUES ($1, $2, $3, $4);

-- name: InsertSecretVersionFirst :exec
INSERT INTO secret_versions (id, secret_id, seq, wrapped_key, nonce, ciphertext)
VALUES ($1, $2, 1, $3, $4, $5);

-- name: InsertDefaultFileBinding :exec
INSERT INTO file_bindings (id, secret_id, path, uid, gid, mode)
VALUES (gen_random_uuid(), $1, $2, 0, 0, 0o400);

-- name: InsertDraft :exec
INSERT INTO drafts (id, application_id, environment_id)
VALUES (gen_random_uuid(), $1, $2)
ON CONFLICT (application_id, environment_id) DO NOTHING;

-- name: UpsertDraftSelection :exec
INSERT INTO draft_selections (draft_id, secret_id, version_seq)
VALUES ($1, $2, $3)
ON CONFLICT (draft_id, secret_id) DO UPDATE SET version_seq = $3;

-- name: InsertDraftSelectionForNewSecret :exec
INSERT INTO draft_selections (draft_id, secret_id, version_seq)
SELECT d.id, $1, 1 FROM drafts d
WHERE d.application_id = $2 AND d.environment_id = $3
ON CONFLICT (draft_id, secret_id) DO UPDATE SET version_seq = 1;

-- name: SelectSecretAppEnv :one
SELECT application_id, environment_id FROM secrets WHERE id = $1;

-- name: InsertSecretVersionNext :one
INSERT INTO secret_versions (id, secret_id, seq, wrapped_key, nonce, ciphertext)
SELECT $1, $2, COALESCE(max(seq), 0) + 1, $3, $4, $5 FROM secret_versions WHERE secret_id = $2
RETURNING seq;

-- name: UpdateDraftSelectionVersion :exec
UPDATE draft_selections ds SET version_seq = $1
FROM drafts d
WHERE ds.draft_id = d.id AND d.application_id = $2 AND d.environment_id = $3 AND ds.secret_id = $4;

-- name: UpdateFileBinding :exec
UPDATE file_bindings SET path = $1, uid = $2, gid = $3, mode = $4, updated_at = now()
WHERE secret_id = $5;

-- name: SelectDraftID :one
SELECT id FROM drafts WHERE application_id = $1 AND environment_id = $2;

-- name: SelectDraftVersion :one
SELECT version FROM drafts WHERE id = $1;

-- name: BumpDraft :one
UPDATE drafts SET version = version + 1, updated_at = now() WHERE id = $1 RETURNING version;

-- name: ListDraftSelections :many
SELECT sc.id, sc.name, ds.version_seq,
       COALESCE(fb.path, ''), COALESCE(fb.uid, 0), COALESCE(fb.gid, 0), COALESCE(fb.mode, 0)
FROM draft_selections ds
JOIN drafts d ON d.id = ds.draft_id
JOIN secrets sc ON sc.id = ds.secret_id
LEFT JOIN file_bindings fb ON fb.secret_id = sc.id
WHERE d.application_id = $1 AND d.environment_id = $2
ORDER BY sc.name;

-- name: SelectDraftIDForUpdate :one
SELECT id FROM drafts WHERE application_id = $1 AND environment_id = $2 FOR UPDATE;

-- name: SelectDraftSelectionExists :one
SELECT EXISTS (SELECT 1 FROM secret_versions sv
    JOIN secrets sc ON sc.id = sv.secret_id
    JOIN drafts d ON d.application_id = sc.application_id AND d.environment_id = sc.environment_id
    WHERE d.id = $1 AND sv.secret_id = $2 AND sv.seq = $3);

-- name: SelectDraftIDVersion :one
SELECT id, version FROM drafts WHERE application_id = $1 AND environment_id = $2;

-- name: CountDraftSelections :one
SELECT count(*) FROM draft_selections WHERE draft_id = $1;

-- name: InsertBundleRevision :exec
INSERT INTO bundle_revisions (id, application_id, environment_id, draft_version, created_by,
    operation_reason_category, operation_reason_explanation, operation_reason_external_ref)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: InsertRevisionFilesFromDraft :exec
INSERT INTO revision_files (revision_id, secret_id, path, uid, gid, mode, version_seq)
SELECT $1, ds.secret_id, fb.path, fb.uid, fb.gid, fb.mode, ds.version_seq
FROM draft_selections ds
JOIN file_bindings fb ON fb.secret_id = ds.secret_id
WHERE ds.draft_id = $2;

-- name: ListRevisions :many
SELECT br.id, br.draft_version, br.created_by,
       br.operation_reason_category, br.operation_reason_explanation, br.operation_reason_external_ref,
       to_char(br.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       (SELECT count(*) FROM revision_files rf WHERE rf.revision_id = br.id)
FROM bundle_revisions br
WHERE br.application_id = $1 AND br.environment_id = $2
ORDER BY br.created_at DESC, br.id DESC;

-- name: ListAllRevisions :many
SELECT br.id, br.draft_version, br.created_by,
       br.operation_reason_category, br.operation_reason_explanation, br.operation_reason_external_ref,
       to_char(br.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       (SELECT count(*) FROM revision_files rf WHERE rf.revision_id = br.id)
FROM bundle_revisions br
ORDER BY br.created_at DESC, br.id DESC
LIMIT $1;

-- name: SelectRevisionDraftVersion :one
SELECT draft_version FROM bundle_revisions
WHERE id = $1 AND application_id = $2 AND environment_id = $3;

-- name: CountRevisionFiles :one
SELECT count(*) FROM revision_files WHERE revision_id = $1;

-- name: InsertRevisionFilesCopy :exec
INSERT INTO revision_files (revision_id, secret_id, path, uid, gid, mode, version_seq)
SELECT $1, rf.secret_id, rf.path, rf.uid, rf.gid, rf.mode, rf.version_seq
FROM revision_files rf WHERE rf.revision_id = $2;

-- name: ListNodeGroups :many
SELECT ng.id, ng.name, COALESCE(array_agg(gm.node_id) FILTER (WHERE gm.node_id IS NOT NULL), '{}')::text[] AS member_ids
FROM node_groups ng
LEFT JOIN group_members gm ON gm.group_id = ng.id
GROUP BY ng.id ORDER BY ng.name;

-- name: CreateNodeGroup :exec
INSERT INTO node_groups (id, name) VALUES ($1, $2);

-- name: CountActiveAssignmentsForNodeGroup :one
SELECT count(*) FROM assignments
WHERE group_id = $1 AND status = 'active';

-- name: DeleteNodeGroupByID :execrows
DELETE FROM node_groups WHERE id = $1;

-- name: InsertGroupMember :exec
INSERT INTO group_members (group_id, node_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: SelectGroupMemberConflict :one
SELECT EXISTS (
    SELECT 1 FROM assignments a
    WHERE a.group_id = $1 AND a.status = 'active'
      AND EXISTS (
        SELECT 1 FROM assignments a2
        JOIN group_members gm2 ON gm2.group_id = a2.group_id
        WHERE gm2.node_id = $2 AND a2.status = 'active'
          AND a2.application_id = a.application_id
          AND a2.environment_id = a.environment_id
          AND a2.group_id <> a.group_id
      )
);

-- name: RemoveGroupMember :exec
DELETE FROM group_members WHERE group_id = $1 AND node_id = $2;

-- name: ListAssignments :many
SELECT a.id, a.group_id, ng.name, a.application_id, a.environment_id, a.revision_id, a.status,
       to_char(a.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM assignments a JOIN node_groups ng ON ng.id = a.group_id
ORDER BY a.created_at DESC, a.id DESC;

-- name: SelectNodeGroupName :one
SELECT name FROM node_groups WHERE id = $1;

-- name: SelectEnvironmentExists :one
SELECT EXISTS (SELECT 1 FROM environments WHERE id = $1 AND application_id = $2);

-- name: SelectLatestRevision :one
SELECT id FROM bundle_revisions
WHERE application_id = $1 AND environment_id = $2
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: SelectAssignmentConflict :one
SELECT EXISTS (
    SELECT 1 FROM group_members gm
    JOIN assignments a2 ON a2.group_id = gm.group_id
      AND a2.application_id = $1 AND a2.environment_id = $2
      AND a2.status = 'active' AND a2.group_id <> $3
    WHERE gm.node_id IN (SELECT node_id FROM group_members WHERE group_id = $3)
);

-- name: InsertAssignment :one
INSERT INTO assignments (id, group_id, application_id, environment_id, revision_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"');

-- name: AdvanceDesiredRevision :exec
UPDATE assignments SET revision_id = $3
WHERE application_id = $1 AND environment_id = $2 AND status = 'active';

-- name: ListNode :many
SELECT id, name, serial, created_at, last_seen_at, desired_etag, observed_revision, last_result,
       poll_interval_seconds, bundle_dir
FROM nodes ORDER BY name;

-- name: GetNode :one
SELECT id, name, serial, created_at, last_seen_at, desired_etag, observed_revision, last_result,
       poll_interval_seconds, bundle_dir
FROM nodes WHERE id = $1;

-- name: NodeBySerial :one
SELECT id, name, serial, age_pubkey, created_at, last_seen_at, desired_etag, observed_revision, last_result,
       poll_interval_seconds, bundle_dir
FROM nodes WHERE serial = $1;

-- name: TouchNode :exec
UPDATE nodes SET last_seen_at = $4, observed_revision = $2, last_result = $3
WHERE id = $1;

-- name: SetNodeDesired :exec
UPDATE nodes SET desired_etag = $2 WHERE id = $1;

-- name: SetNodePollInterval :execrows
UPDATE nodes SET poll_interval_seconds = $2 WHERE id = $1;

-- name: RenameNode :execrows
UPDATE nodes SET name = $2 WHERE id = $1;

-- name: SetNodeBundleDir :execrows
UPDATE nodes SET bundle_dir = $2 WHERE id = $1;

-- name: DeleteNode :execrows
DELETE FROM nodes WHERE id = $1;

-- name: CreateEnrollmentToken :exec
INSERT INTO enrollment_tokens (token_hash, name, expires_at) VALUES ($1, $2, $3);

-- name: CreateNodeEnrollmentToken :exec
INSERT INTO enrollment_tokens (token_hash, name, expires_at, node_id)
VALUES ($1, $2, $3, $4);

-- name: SelectEnrollmentTokenForUpdate :one
SELECT token_hash, name, created_at, expires_at, node_id
FROM enrollment_tokens
WHERE token_hash = $1 FOR UPDATE;

-- name: SelectEnrollmentTokenUsedAt :one
SELECT used_at FROM enrollment_tokens WHERE token_hash = $1;

-- name: MarkEnrollmentTokenUsed :exec
UPDATE enrollment_tokens SET used_at = $2 WHERE token_hash = $1;

-- name: RegisterNode :exec
INSERT INTO nodes (id, name, serial, age_pubkey, cert_pem, cert_expires_at, bundle_dir)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ActivatePendingNode :execrows
UPDATE nodes SET serial = $2, age_pubkey = $3, cert_pem = $4, cert_expires_at = $5
WHERE id = $1;

-- name: AssignedRevisions :many
SELECT DISTINCT a.revision_id
FROM assignments a
JOIN group_members gm ON gm.group_id = a.group_id
WHERE gm.node_id = $1
  AND a.status = 'active'
ORDER BY a.revision_id;

-- name: RevisionFiles :many
SELECT revision_id, secret_id, path, uid, gid, mode, version_seq
FROM revision_files WHERE revision_id = $1 ORDER BY path;

-- name: RevisionAppEnv :one
SELECT application_id, environment_id FROM bundle_revisions WHERE id = $1;

-- name: SecretVersionBlob :one
SELECT wrapped_key, nonce, ciphertext FROM secret_versions
WHERE secret_id = $1 AND seq = $2;

-- name: SecretAppEnv :one
SELECT application_id, environment_id FROM secrets WHERE id = $1;

-- name: CountActiveAssignmentsForEnvironment :one
SELECT count(*) FROM assignments
WHERE application_id = $1 AND environment_id = $2 AND status = 'active';

-- name: CountActiveAssignmentsForApplication :one
SELECT count(*) FROM assignments
WHERE application_id = $1 AND status = 'active';

-- name: CountActiveAssignmentsForSecret :one
SELECT count(*)
FROM assignments a
JOIN secrets s ON s.application_id = a.application_id AND s.environment_id = a.environment_id
WHERE s.id = $1 AND a.status = 'active';

-- name: DeleteRevisionFilesForSecret :exec
DELETE FROM revision_files WHERE secret_id = $1;

-- name: DeleteSecretByID :execrows
DELETE FROM secrets WHERE id = $1;

-- name: DeleteRevisionFilesForEnvironment :exec
DELETE FROM revision_files
WHERE revision_id IN (
  SELECT id FROM bundle_revisions WHERE application_id = $1 AND environment_id = $2
);

-- name: DeleteEnvironmentByID :execrows
DELETE FROM environments WHERE id = $1 AND application_id = $2;

-- name: DeleteRevisionFilesForApplication :exec
DELETE FROM revision_files
WHERE revision_id IN (
  SELECT id FROM bundle_revisions WHERE application_id = $1
);

-- name: DeleteApplicationByID :execrows
DELETE FROM applications WHERE id = $1;
