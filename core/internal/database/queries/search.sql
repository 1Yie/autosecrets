-- name: SearchApplications :many
SELECT id, name FROM applications WHERE name ILIKE $1 ORDER BY name LIMIT 20;

-- name: SearchEnvironments :many
SELECT e.id, (a.name || '/' || e.name)::text AS name FROM environments e
JOIN applications a ON a.id = e.application_id
WHERE e.name ILIKE $1 ORDER BY e.name LIMIT 20;

-- name: SearchNodes :many
SELECT id, name FROM nodes WHERE name ILIKE $1 OR serial ILIKE $1 ORDER BY name LIMIT 20;

-- name: SearchNodeGroups :many
SELECT id, name FROM node_groups WHERE name ILIKE $1 ORDER BY name LIMIT 20;
