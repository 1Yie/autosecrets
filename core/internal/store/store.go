// Package store owns PostgreSQL access: the embedded schema, migration
// application, and every query the Core service needs. There are no
// repository layers for mocking; tests exercise the store through the HTTP
// seam against a real PostgreSQL instance.
package store

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

// Exec runs a statement on the pool (used by tests and migrations).
func (s *Store) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return s.pool.Exec(ctx, sql, args...)
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("store: migrations table: %w", err)
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		var applied bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("store: migrate %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

// --- Compatibility identity APIs ------------------------------------------
//
// The original vertical slice exposed the first Organization Member as an
// Admin internally. Keep these wrappers until all callers use Member names;
// the stored schema is extended by 0003_identity_security.sql.

func (s *Store) AdminCount(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM admins`).Scan(&n)
	return n, err
}

func (s *Store) AdminByUsername(ctx context.Context, username string) (*Admin, error) {
	return s.MemberByUsername(ctx, username)
}

func (s *Store) CreateAdmin(ctx context.Context, id, username, passwordHash string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO admins (id, username, password_hash, role, status, activated_at)
		 VALUES ($1, $2, $3, $4, $5, now())`,
		id, username, passwordHash, RoleAdministrator, MemberActive)
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}

// SaveBootstrapCode stores the hash of an unused bootstrap code.
func (s *Store) SaveBootstrapCode(ctx context.Context, codeHash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO bootstrap_codes (code_hash, expires_at) VALUES ($1, $2)`, codeHash, expiresAt)
	return err
}

// ConsumeBootstrapCode atomically marks a code used and reports whether it
// was valid (existed, unexpired, unused).
func (s *Store) ConsumeBootstrapCode(ctx context.Context, codeHash string, now time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE bootstrap_codes SET used_at = $2
		 WHERE code_hash = $1 AND used_at IS NULL AND expires_at > $2`, codeHash, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// --- Sessions -------------------------------------------------------------

// CreateSession is retained for the original lifecycle tests. New callers
// use CreateBoundedSession so both Session limits are explicit at the call
// site.
func (s *Store) CreateSession(ctx context.Context, idHash, adminID, csrfToken string, expiresAt time.Time) error {
	idleExpiresAt := time.Now().Add(30 * time.Minute)
	if idleExpiresAt.After(expiresAt) {
		idleExpiresAt = expiresAt
	}
	return s.CreateBoundedSession(ctx, idHash, adminID, csrfToken, expiresAt, idleExpiresAt)
}

// SessionRow is the materialized session state for one request.
type SessionRow struct {
	AdminID       string
	Username      string
	Role          string
	CSRFToken     string
	SessionIDHash string
	ExpiresAt     time.Time
	IdleExpiresAt time.Time
}

func (s *Store) SessionByID(ctx context.Context, idHash string, now time.Time) (*SessionRow, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT s.admin_id, a.username, a.role, s.csrf_token, s.id_hash, s.expires_at, s.idle_expires_at
		 FROM sessions s JOIN admins a ON a.id = s.admin_id
		 WHERE s.id_hash = $1 AND s.expires_at > $2 AND s.idle_expires_at > $2
		   AND a.status = $3`, idHash, now, MemberActive)
	var sr SessionRow
	if err := row.Scan(&sr.AdminID, &sr.Username, &sr.Role, &sr.CSRFToken,
		&sr.SessionIDHash, &sr.ExpiresAt, &sr.IdleExpiresAt); err != nil {
		return nil, mapNoRows(err)
	}
	return &sr, nil
}

func (s *Store) DeleteSession(ctx context.Context, idHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id_hash = $1`, idHash)
	return err
}

// --- Audit ----------------------------------------------------------------

type AuditEvent struct {
	ID                int64  `json:"id"`
	Actor             string `json:"actor"`
	Action            string `json:"action"`
	Resource          string `json:"resource"`
	Result            string `json:"result"`
	CorrelationID     string `json:"correlation_id"`
	CreatedAt         string `json:"created_at"`
	ActorType         string `json:"actor_type"`
	ActorID           string `json:"actor_id"`
	ActorDisplay      string `json:"actor_display"`
	ResourceType      string `json:"resource_type"`
	ResourceID        string `json:"resource_id"`
	ResourceDisplay   string `json:"resource_display"`
	Outcome           string `json:"outcome"`
	ReasonCategory    string `json:"operation_reason_category"`
	ReasonExplanation string `json:"operation_reason_explanation"`
	ReasonExternalRef string `json:"operation_reason_external_ref"`
}

// AppendAudit inserts an Audit Event inside the caller's transaction when tx
// is non-nil, otherwise in its own transaction. The Audit Event is
// append-only; nothing in this package ever updates or deletes one.
func (s *Store) AppendAudit(ctx context.Context, tx pgx.Tx, e AuditEvent) error {
	execer := execer(ctx, s.pool, tx)
	_, err := execer.Exec(ctx,
		`INSERT INTO audit_events
			(actor, action, resource, result, correlation_id,
			 actor_type, actor_id, actor_display,
			 resource_type, resource_id, resource_display,
			 outcome, operation_reason_category, operation_reason_explanation, operation_reason_external_ref)
		 VALUES ($1, $2, $3, $4, $5,
			 COALESCE(NULLIF($6, ''), split_part($1, ':', 1)),
			 COALESCE(NULLIF($7, ''), split_part($1, ':', 2)),
			 COALESCE(NULLIF($8, ''), $1),
			 COALESCE(NULLIF($9, ''), split_part($3, ':', 1)),
			 COALESCE(NULLIF($10, ''), split_part($3, ':', 2)),
			 COALESCE(NULLIF($11, ''), $3),
			 COALESCE(NULLIF($12, ''), $4),
			 COALESCE(NULLIF($13, ''), ''), COALESCE(NULLIF($14, ''), ''), COALESCE(NULLIF($15, ''), ''))`,
		e.Actor, e.Action, e.Resource, e.Result, e.CorrelationID,
		e.ActorType, e.ActorID, e.ActorDisplay,
		e.ResourceType, e.ResourceID, e.ResourceDisplay,
		e.Outcome, e.ReasonCategory, e.ReasonExplanation, e.ReasonExternalRef)
	return err
}

type AuditFilter struct {
	Actor          string
	Action         string
	Resource       string
	Outcome        string
	ReasonCategory string
	From           time.Time
	To             time.Time
	Limit          int
}

func (s *Store) ListAudit(ctx context.Context, f AuditFilter) ([]AuditEvent, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	query := `SELECT id, actor, action, resource, result, correlation_id,
		to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		FROM audit_events`
	args := []any{}
	conds := []string{}
	if f.Actor != "" {
		args = append(args, f.Actor)
		conds = append(conds, fmt.Sprintf("actor = $%d", len(args)))
	}
	if f.Action != "" {
		args = append(args, f.Action)
		conds = append(conds, fmt.Sprintf("action = $%d", len(args)))
	}
	for i, c := range conds {
		if i == 0 {
			query += " WHERE " + c
		} else {
			query += " AND " + c
		}
	}
	args = append(args, f.Limit)
	query += fmt.Sprintf(" ORDER BY id DESC LIMIT $%d", len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Resource, &e.Result, &e.CorrelationID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListAuditPage returns cursor-paginated structured Audit Events ordered by
// id DESC with the documented filters (ADR-0020). The cursor is the last
// visible event id.
func (s *Store) ListAuditPage(ctx context.Context, f AuditFilter, afterID int64) ([]AuditEvent, string, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 25
	}
	query := `SELECT id, actor, action, resource, result, correlation_id,
		to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		actor_type, actor_id, actor_display, resource_type, resource_id, resource_display,
		outcome, operation_reason_category, operation_reason_explanation, operation_reason_external_ref
		FROM audit_events`
	args := []any{}
	conds := []string{}
	if afterID > 0 {
		args = append(args, afterID)
		conds = append(conds, fmt.Sprintf("id < $%d", len(args)))
	}
	add := func(column, value string) {
		if value == "" {
			return
		}
		args = append(args, value)
		conds = append(conds, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	add("actor", f.Actor)
	add("action", f.Action)
	add("resource", f.Resource)
	add("outcome", f.Outcome)
	add("operation_reason_category", f.ReasonCategory)
	if !f.From.IsZero() {
		args = append(args, f.From)
		conds = append(conds, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !f.To.IsZero() {
		args = append(args, f.To)
		conds = append(conds, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, f.Limit+1)
	query += fmt.Sprintf(" ORDER BY id DESC LIMIT $%d", len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Resource, &e.Result, &e.CorrelationID,
			&e.CreatedAt, &e.ActorType, &e.ActorID, &e.ActorDisplay,
			&e.ResourceType, &e.ResourceID, &e.ResourceDisplay, &e.Outcome,
			&e.ReasonCategory, &e.ReasonExplanation, &e.ReasonExternalRef); err != nil {
			return nil, "", err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(out) <= f.Limit {
		return out, "", nil
	}
	next := out[f.Limit]
	return out[:f.Limit], strconv.FormatInt(next.ID, 10), nil
}

// --- transactions ---------------------------------------------------------

func (s *Store) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}

func execer(ctx context.Context, pool *pgxpool.Pool, tx pgx.Tx) interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
} {
	if tx != nil {
		return tx
	}
	return pool
}
