package migrations_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"autosecrets.dev/core/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOptionalTOTPMigrationPreservesOnlyConfirmedEnrollment(t *testing.T) {
	for _, tc := range []struct {
		name            string
		confirmed       bool
		wantPolicy      bool
		wantEnrollments int
		wantRecovery    int
	}{
		{name: "confirmed enrollment", confirmed: true, wantPolicy: true, wantEnrollments: 1, wantRecovery: 1},
		{name: "pending enrollment", confirmed: false, wantPolicy: false, wantEnrollments: 0, wantRecovery: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := migrationPool(t)
			ctx := context.Background()
			applyThrough(t, ctx, pool, "0010_drop_secret_rotations.sql")

			adminID := uuid.NewString()
			status := "active"
			if !tc.confirmed {
				status = "pending"
			}
			if _, err := pool.Exec(ctx, `INSERT INTO admins
				(id, username, password_hash, role, status) VALUES ($1, 'admin', 'hash', 'administrator', $2)`, adminID, status); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO organization_config (display_name) VALUES ('Acme')`); err != nil {
				t.Fatal(err)
			}
			verified := "NULL"
			confirmed := "NULL"
			if tc.confirmed {
				verified = "now()"
				confirmed = "now()"
			}
			if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO mfa_enrollments
				(token_hash, admin_id, wrapped_key, nonces, ciphertext, expires_at, verified_at, confirmed_at)
				VALUES ('token', $1, '\x01', '\x02', '\x03', now() + interval '1 hour', %s, %s)`, verified, confirmed), adminID); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO recovery_codes (admin_id, code_hash) VALUES ($1, 'recovery')`, adminID); err != nil {
				t.Fatal(err)
			}

			applyMigration(t, ctx, pool, "0011_single_administrator_optional_totp_oidc.sql")

			var policy bool
			var migratedStatus string
			var migrationOutcome string
			var enrollmentCount, recoveryCount int
			if err := pool.QueryRow(ctx, `SELECT totp_login_required FROM organization_config WHERE singleton`).Scan(&policy); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT status FROM admins WHERE id = $1`, adminID).Scan(&migratedStatus); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM mfa_enrollments`).Scan(&enrollmentCount); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM recovery_codes`).Scan(&recoveryCount); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT result FROM audit_events WHERE action = 'identity.migrated'`).Scan(&migrationOutcome); err != nil {
				t.Fatal(err)
			}
			wantOutcome := "password_only_enabled"
			if tc.wantPolicy {
				wantOutcome = "totp_preserved"
			}
			if policy != tc.wantPolicy || migratedStatus != "active" || enrollmentCount != tc.wantEnrollments || recoveryCount != tc.wantRecovery {
				t.Fatalf("migration result: policy=%v status=%s enrollments=%d recovery=%d", policy, migratedStatus, enrollmentCount, recoveryCount)
			}
			if migrationOutcome != wantOutcome {
				t.Fatalf("migration audit outcome=%q, want %q", migrationOutcome, wantOutcome)
			}
		})
	}
}

func migrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	schema := "migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := pool.Exec(context.Background(), `CREATE SCHEMA `+schema); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `SET search_path TO `+schema); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		pool.Close()
	})
	return pool
}

func applyThrough(t *testing.T, ctx context.Context, pool *pgxpool.Pool, last string) {
	t.Helper()
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".sql") && entry.Name() <= last {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		applyMigration(t, ctx, pool, name)
	}
}

func applyMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) {
	t.Helper()
	sql, err := migrations.FS.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("apply %s: %v", name, err)
	}
}
