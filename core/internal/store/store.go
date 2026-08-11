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

// --- Admins and bootstrap -------------------------------------------------

type Admin struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

func (s *Store) AdminCount(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM admins`).Scan(&n)
	return n, err
}

func (s *Store) AdminByUsername(ctx context.Context, username string) (*Admin, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at FROM admins WHERE username = $1`, username)
	return scanAdmin(row)
}

func (s *Store) CreateAdmin(ctx context.Context, id, username, passwordHash string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO admins (id, username, password_hash) VALUES ($1, $2, $3)`,
		id, username, passwordHash)
	return err
}

func scanAdmin(row pgx.Row) (*Admin, error) {
	a := &Admin{}
	if err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.CreatedAt); err != nil {
		return nil, err
	}
	return a, nil
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

func (s *Store) CreateSession(ctx context.Context, idHash, adminID, csrfToken string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (id_hash, admin_id, csrf_token, expires_at) VALUES ($1, $2, $3, $4)`,
		idHash, adminID, csrfToken, expiresAt)
	return err
}

// SessionRow is the materialized session state for one request.
type SessionRow struct {
	AdminID  string
	Username string
	CSRFToken string
}

func (s *Store) SessionByID(ctx context.Context, idHash string, now time.Time) (*SessionRow, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT s.admin_id, a.username, s.csrf_token
		 FROM sessions s JOIN admins a ON a.id = s.admin_id
		 WHERE s.id_hash = $1 AND s.expires_at > $2`, idHash, now)
	var sr SessionRow
	if err := row.Scan(&sr.AdminID, &sr.Username, &sr.CSRFToken); err != nil {
		return nil, err
	}
	return &sr, nil
}

func (s *Store) DeleteSession(ctx context.Context, idHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id_hash = $1`, idHash)
	return err
}

// --- Audit ----------------------------------------------------------------

type AuditEvent struct {
	ID            int64  `json:"id"`
	Actor         string `json:"actor"`
	Action        string `json:"action"`
	Resource      string `json:"resource"`
	Result        string `json:"result"`
	CorrelationID string `json:"correlation_id"`
	CreatedAt     string `json:"created_at"`
}

// AppendAudit inserts an Audit Event inside the caller's transaction when tx
// is non-nil, otherwise in its own transaction. The Audit Event is
// append-only; nothing in this package ever updates or deletes one.
func (s *Store) AppendAudit(ctx context.Context, tx pgx.Tx, e AuditEvent) error {
	execer := execer(ctx, s.pool, tx)
	_, err := execer.Exec(ctx,
		`INSERT INTO audit_events (actor, action, resource, result, correlation_id)
		 VALUES ($1, $2, $3, $4, $5)`,
		e.Actor, e.Action, e.Resource, e.Result, e.CorrelationID)
	return err
}

type AuditFilter struct {
	Actor  string
	Action string
	Limit  int
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
