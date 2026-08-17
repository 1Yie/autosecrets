-- name: OverviewCounts :one
SELECT
    (SELECT count(*) FROM applications) AS applications,
    (SELECT count(*) FROM environments) AS environments,
    (
        SELECT count(*) FROM secrets
        WHERE retired_at IS NULL
    ) AS secrets,
    (SELECT count(*) FROM nodes) AS nodes,
    (SELECT count(*) FROM node_groups) AS node_groups,
    (
        SELECT count(*) FROM assignments
        WHERE status = 'active'
    ) AS assignments,
    (SELECT count(*) FROM audit_events) AS audit_events;

-- name: OverviewAttentionCounts :one
SELECT
    (
        SELECT count(DISTINCT node_id) FROM node_convergence
        WHERE result = 'failed'
    ) AS failed,
    (
        SELECT count(*) FROM nodes
        WHERE
            last_seen_at IS NULL OR last_seen_at < now() - INTERVAL '75 seconds'
    ) AS offline,
    (
        SELECT count(*) FROM nodes AS n
        WHERE NOT EXISTS (
            SELECT 1
            FROM group_members AS gm
            INNER JOIN assignments AS a ON gm.group_id = a.group_id
            WHERE gm.node_id = n.id AND a.status = 'active'
        )
    ) AS unassigned_nodes;

-- name: OverviewCleanupCounts :one
SELECT
    (
        SELECT count(*) FROM unassignment_tasks
        WHERE status = 'failed')
        AS cleanup_failed,
    (
        SELECT count(*) FROM unassignment_tasks
        WHERE status = 'cleanup_unconfirmed'
    ) AS cleanup_unconfirmed;
