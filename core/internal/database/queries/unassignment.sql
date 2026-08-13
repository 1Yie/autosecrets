-- name: UpsertActivationPolicy :exec
INSERT INTO activation_policies (environment_id, action)
VALUES ($1, $2)
ON CONFLICT (environment_id) DO UPDATE SET action = EXCLUDED.action, updated_at = now();

-- name: DeleteActivationPolicyUnits :exec
DELETE FROM activation_policy_units WHERE environment_id = $1;

-- name: InsertActivationPolicyUnit :exec
INSERT INTO activation_policy_units (environment_id, position, unit_name) VALUES ($1, $2, $3);

-- name: GetActivationPolicy :one
SELECT environment_id, action FROM activation_policies WHERE environment_id = $1;

-- name: ListActivationPolicyUnits :many
SELECT unit_name FROM activation_policy_units WHERE environment_id = $1 ORDER BY position;

-- name: MarkAssignmentRemoving :execrows
UPDATE assignments SET status = 'removing' WHERE id = $1 AND status = 'active';

-- name: InsertUnassignmentTasks :exec
INSERT INTO unassignment_tasks (assignment_id, node_id)
SELECT $1, gm.node_id FROM group_members gm WHERE gm.group_id = (
    SELECT group_id FROM assignments WHERE id = $1);

-- name: ListUnassignmentTasks :many
SELECT assignment_id, node_id, status, error, updated_at
FROM unassignment_tasks WHERE assignment_id = $1 ORDER BY node_id;

-- name: ReportCleanupTask :execrows
UPDATE unassignment_tasks
SET status = $3, error = $4, updated_at = now()
WHERE assignment_id = $1 AND node_id = $2
  AND status IN ('pending', 'failed', 'cleanup_unconfirmed');

-- name: AbandonCleanupConfirmation :execrows
UPDATE unassignment_tasks SET status = 'cleanup_unconfirmed', updated_at = now()
WHERE assignment_id = $1 AND status IN ('pending', 'failed', 'offline');

-- name: AssignmentByID :one
SELECT a.id, a.group_id, ng.name, a.application_id, a.environment_id, a.revision_id, a.status,
       to_char(a.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
FROM assignments a JOIN node_groups ng ON ng.id = a.group_id
WHERE a.id = $1;

-- name: PendingCleanupInstructions :many
SELECT t.assignment_id, a.application_id, a.environment_id
FROM unassignment_tasks t
JOIN assignments a ON a.id = t.assignment_id
WHERE t.node_id = $1 AND t.status IN ('pending', 'failed', 'cleanup_unconfirmed')
ORDER BY t.assignment_id;
