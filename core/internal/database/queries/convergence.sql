-- name: RecordConvergence :exec
INSERT INTO node_convergence
    (node_id, assignment_id, application_id, environment_id,
     desired_revision, observed_revision, stage, result, error, reported_at)
VALUES ($1, $2, $3, $4, $5,
    CASE WHEN $8 = 'ok' THEN $6::text ELSE '' END, $7, $8, $9, $10)
ON CONFLICT (node_id, assignment_id) DO UPDATE SET
    application_id = EXCLUDED.application_id,
    environment_id = EXCLUDED.environment_id,
    desired_revision = EXCLUDED.desired_revision,
    observed_revision = CASE
        WHEN EXCLUDED.result = 'ok' THEN EXCLUDED.observed_revision
        ELSE node_convergence.observed_revision END,
    stage = EXCLUDED.stage,
    result = EXCLUDED.result,
    error = EXCLUDED.error,
    reported_at = EXCLUDED.reported_at,
    updated_at = now();

-- name: AssignmentForNodeRevision :one
SELECT a.id, a.group_id, ng.name, a.application_id, a.environment_id, a.revision_id, a.status,
       to_char(a.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
FROM assignments a
JOIN group_members gm ON gm.group_id = a.group_id
JOIN node_groups ng ON ng.id = a.group_id
WHERE gm.node_id = $1 AND a.revision_id = $2 AND a.status = 'active'
LIMIT 1;

-- name: NodeConvergence :many
SELECT node_id, assignment_id, application_id, environment_id,
       desired_revision, observed_revision, stage, result, error, reported_at
FROM node_convergence WHERE node_id = $1
ORDER BY (result = 'failed') DESC, updated_at DESC;

-- name: AllNodeConvergence :many
SELECT node_id, assignment_id, application_id, environment_id,
       desired_revision, observed_revision, stage, result, error, reported_at
FROM node_convergence ORDER BY node_id;

-- name: NodeAssignmentCounts :many
SELECT gm.node_id, count(*)::int AS count
FROM group_members gm
JOIN assignments a ON a.group_id = gm.group_id
WHERE a.status = 'active'
GROUP BY gm.node_id;
