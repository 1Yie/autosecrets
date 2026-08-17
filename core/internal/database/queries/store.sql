-- name: AdminCount :one
SELECT count(*) FROM admins;

-- name: CreateAdmin :exec
INSERT INTO admins (id, username, password_hash, role, status, activated_at)
VALUES ($1, $2, $3, $4, $5, now());

-- name: SaveBootstrapCode :exec
INSERT INTO bootstrap_codes (code_hash, expires_at) VALUES ($1, $2);

-- name: ConsumeBootstrapCode :execrows
UPDATE bootstrap_codes SET used_at = $2
WHERE code_hash = $1 AND used_at IS NULL AND expires_at > $2;

-- name: SessionByID :one
SELECT s.admin_id, a.username, a.role, s.csrf_token, s.id_hash, s.expires_at, s.idle_expires_at, s.auth_method
FROM sessions s JOIN admins a ON a.id = s.admin_id
WHERE s.id_hash = $1 AND s.expires_at > $2 AND s.idle_expires_at > $2
  AND a.status = $3;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id_hash = $1;

-- name: InsertAuditEvent :exec
INSERT INTO audit_events
    (actor, action, resource, result, correlation_id,
     actor_type, actor_id, actor_display,
     resource_type, resource_id, resource_display,
     outcome, operation_reason_category, operation_reason_explanation, operation_reason_external_ref)
VALUES ($1, $2, $3, $4, $5,
     COALESCE(NULLIF($6::text, ''), split_part($1, ':', 1)),
     COALESCE(NULLIF($7::text, ''), split_part($1, ':', 2)),
     COALESCE(NULLIF($8::text, ''), $1),
     COALESCE(NULLIF($9::text, ''), split_part($3, ':', 1)),
     COALESCE(NULLIF($10::text, ''), split_part($3, ':', 2)),
     COALESCE(NULLIF($11::text, ''), $3),
     COALESCE(NULLIF($12::text, ''), $4),
     COALESCE(NULLIF($13::text, ''), ''), COALESCE(NULLIF($14::text, ''), ''), COALESCE(NULLIF($15::text, ''), ''));

-- name: ListAudit :many
SELECT id, actor, action, resource, result, correlation_id,
    to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
FROM audit_events
WHERE (actor = sqlc.narg('actor') OR sqlc.narg('actor') IS NULL)
  AND (action = sqlc.narg('action') OR sqlc.narg('action') IS NULL)
ORDER BY id DESC
LIMIT sqlc.arg('limit');

-- name: ListAuditPage :many
SELECT id, actor, action, resource, result, correlation_id,
    to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at,
    actor_type, actor_id, actor_display, resource_type, resource_id, resource_display,
    outcome, operation_reason_category, operation_reason_explanation, operation_reason_external_ref
FROM audit_events
WHERE (id < sqlc.narg('after_id') OR sqlc.narg('after_id') IS NULL)
  AND (actor = sqlc.narg('actor') OR sqlc.narg('actor') IS NULL)
  AND (action = sqlc.narg('action') OR sqlc.narg('action') IS NULL)
  AND (resource = sqlc.narg('resource') OR sqlc.narg('resource') IS NULL)
  AND (outcome = sqlc.narg('outcome') OR sqlc.narg('outcome') IS NULL)
  AND (operation_reason_category = sqlc.narg('reason_category') OR sqlc.narg('reason_category') IS NULL)
  AND (created_at >= sqlc.narg('from') OR sqlc.narg('from') IS NULL)
  AND (created_at <= sqlc.narg('to') OR sqlc.narg('to') IS NULL)
ORDER BY id DESC
LIMIT sqlc.arg('limit');

-- name: CountAuditEvents :one
SELECT count(*) FROM audit_events
WHERE (actor = sqlc.narg('actor') OR sqlc.narg('actor') IS NULL)
  AND (action = sqlc.narg('action') OR sqlc.narg('action') IS NULL)
  AND (resource = sqlc.narg('resource') OR sqlc.narg('resource') IS NULL)
  AND (outcome = sqlc.narg('outcome') OR sqlc.narg('outcome') IS NULL)
  AND (operation_reason_category = sqlc.narg('reason_category') OR sqlc.narg('reason_category') IS NULL)
  AND (created_at >= sqlc.narg('from') OR sqlc.narg('from') IS NULL)
  AND (created_at <= sqlc.narg('to') OR sqlc.narg('to') IS NULL);

-- name: ListAuditOffsetPage :many
SELECT id, actor, action, resource, result, correlation_id,
    to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at,
    actor_type, actor_id, actor_display, resource_type, resource_id, resource_display,
    outcome, operation_reason_category, operation_reason_explanation, operation_reason_external_ref
FROM audit_events
WHERE (actor = sqlc.narg('actor') OR sqlc.narg('actor') IS NULL)
  AND (action = sqlc.narg('action') OR sqlc.narg('action') IS NULL)
  AND (resource = sqlc.narg('resource') OR sqlc.narg('resource') IS NULL)
  AND (outcome = sqlc.narg('outcome') OR sqlc.narg('outcome') IS NULL)
  AND (operation_reason_category = sqlc.narg('reason_category') OR sqlc.narg('reason_category') IS NULL)
  AND (created_at >= sqlc.narg('from') OR sqlc.narg('from') IS NULL)
  AND (created_at <= sqlc.narg('to') OR sqlc.narg('to') IS NULL)
ORDER BY id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
