-- name: CountApplications :one
SELECT count(*) FROM applications;

-- name: CountNodeGroups :one
SELECT count(*) FROM node_groups;

-- name: CountAssignments :one
SELECT count(*) FROM assignments;

-- name: CountNodes :one
SELECT count(*) FROM nodes;

-- name: ListApplicationsFirstPage :many
SELECT id, name, created_at FROM applications
ORDER BY created_at DESC, id DESC LIMIT $1;

-- name: ListApplicationsNextPage :many
SELECT id, name, created_at FROM applications
WHERE (created_at < $1 OR (created_at = $1 AND id < $2))
ORDER BY created_at DESC, id DESC LIMIT $3;

-- name: ListNodeGroupsFirstPage :many
SELECT ng.id, ng.name, ng.created_at,
       COALESCE(array_agg(gm.node_id) FILTER (WHERE gm.node_id IS NOT NULL), '{}')::text[] AS member_ids
FROM node_groups ng
LEFT JOIN group_members gm ON gm.group_id = ng.id
GROUP BY ng.id
ORDER BY ng.created_at DESC, ng.id DESC LIMIT $1;

-- name: ListNodeGroupsNextPage :many
SELECT ng.id, ng.name, ng.created_at,
       COALESCE(array_agg(gm.node_id) FILTER (WHERE gm.node_id IS NOT NULL), '{}')::text[] AS member_ids
FROM node_groups ng
LEFT JOIN group_members gm ON gm.group_id = ng.id
WHERE (ng.created_at < $1 OR (ng.created_at = $1 AND ng.id < $2))
GROUP BY ng.id
ORDER BY ng.created_at DESC, ng.id DESC LIMIT $3;

-- name: ListAssignmentsFirstPage :many
SELECT a.id, a.group_id, ng.name, a.application_id, a.environment_id, a.revision_id, a.status, a.created_at
FROM assignments a JOIN node_groups ng ON ng.id = a.group_id
ORDER BY a.created_at DESC, a.id DESC LIMIT $1;

-- name: ListAssignmentsNextPage :many
SELECT a.id, a.group_id, ng.name, a.application_id, a.environment_id, a.revision_id, a.status, a.created_at
FROM assignments a JOIN node_groups ng ON ng.id = a.group_id
WHERE (a.created_at < $1 OR (a.created_at = $1 AND a.id < $2))
ORDER BY a.created_at DESC, a.id DESC LIMIT $3;

-- name: ListApplicationsOffsetPage :many
SELECT id, name, created_at FROM applications
ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2;

-- name: ListNodeGroupsOffsetPage :many
SELECT ng.id, ng.name, ng.created_at,
       COALESCE(array_agg(gm.node_id) FILTER (WHERE gm.node_id IS NOT NULL), '{}')::text[] AS member_ids
FROM node_groups ng
LEFT JOIN group_members gm ON gm.group_id = ng.id
GROUP BY ng.id
ORDER BY ng.created_at DESC, ng.id DESC LIMIT $1 OFFSET $2;

-- name: ListAssignmentsOffsetPage :many
SELECT a.id, a.group_id, ng.name, a.application_id, a.environment_id, a.revision_id, a.status, a.created_at
FROM assignments a JOIN node_groups ng ON ng.id = a.group_id
ORDER BY a.created_at DESC, a.id DESC LIMIT $1 OFFSET $2;
