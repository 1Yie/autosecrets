// Package testutil provides shared test infrastructure for Core packages:
// PostgreSQL lifecycle, key material, and schema reset helpers. Every Core
// integration test goes through this package so the harness is defined once
// instead of once per package.
package testutil

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect returns a *database.Store connected to a real PostgreSQL database with
// all migrations applied. The DSN comes from TEST_DATABASE_URL, which
// scripts/test-all.sh and CI export after starting the shared PostgreSQL
// container. Tests fail (not skip) when PostgreSQL is unreachable so a
// missing database can never pass silently.
//
// Every call creates a fresh, uniquely named database on the same server and
// drops it on cleanup. This makes package tests parallel-safe: package app
// and package store (or two `go test ./...` workers) can never clobber each
// other's rows, which a single shared database would.
//
// Deferred: automatic container startup via testcontainers-go. The module
// could not be vendored in the session that introduced this harness
// (module proxy unreachable); once the network allows, wrap this function:
//
//	dsn := os.Getenv("TEST_DATABASE_URL")
//	if dsn == "" { dsn = startContainer(t, ctx) }
func Connect(t *testing.T) *database.Store {
	t.Helper()
	ctx := context.Background()
	template := os.Getenv("TEST_DATABASE_URL")
	if template == "" {
		t.Fatal("TEST_DATABASE_URL is required: start the shared PostgreSQL " +
			"container (scripts/test-all.sh does this) and export the DSN")
	}
	dbName, dsn := createDatabase(t, ctx, template)
	st, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	// Cleanup runs LIFO: close the store's pool first, then drop the database.
	t.Cleanup(func() { dropDatabase(t, ctx, template, dbName) })
	t.Cleanup(st.Close)
	return st
}

// createDatabase creates a uniquely named database on the server behind
// template and returns its name and a DSN pointing at it.
func createDatabase(t *testing.T, ctx context.Context, template string) (string, string) {
	t.Helper()
	dbName := fmt.Sprintf("autosecrets_test_%d_%d", os.Getpid(), time.Now().UnixNano()%1_000_000)
	admin := adminPool(t, ctx, template)
	defer admin.Close()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	u, err := url.Parse(template)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + dbName
	return dbName, u.String()
}

func dropDatabase(t *testing.T, ctx context.Context, template, dbName string) {
	t.Helper()
	admin := adminPool(t, ctx, template)
	defer admin.Close()
	if _, err := admin.Exec(ctx, "DROP DATABASE "+dbName+" WITH (FORCE)"); err != nil {
		// A failed test may already have torn down the server; dropping is
		// best-effort cleanup and must not mask the original failure.
		t.Logf("drop test database %s: %v", dbName, err)
	}
}

func adminPool(t *testing.T, ctx context.Context, template string) *pgxpool.Pool {
	t.Helper()
	u, err := url.Parse(template)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/postgres"
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connect to postgres database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping postgres database: %v", err)
	}
	return pool
}

// NewKeyMaterial loads or creates the master key, Agent CA, and Core signing
// key in a fresh temporary directory, mirroring the production startup path
// (ADR-0003: key material outside PostgreSQL).
func NewKeyMaterial(t *testing.T) (*crypto.MasterKey, *crypto.CA, *crypto.Signer) {
	t.Helper()
	dir := t.TempDir()
	mk, err := crypto.LoadOrCreateMasterKey(dir)
	if err != nil {
		t.Fatalf("master key: %v", err)
	}
	ca, err := crypto.LoadOrCreateCA(dir)
	if err != nil {
		t.Fatalf("agent CA: %v", err)
	}
	signer, err := crypto.LoadOrCreateSigner(dir)
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	return mk, ca, signer
}

// Truncate resets every product table so tests share one database without
// leaking state between them.
func Truncate(t *testing.T, st *database.Store) {
	t.Helper()
	_, err := st.Exec(context.Background(), `TRUNCATE organization_config, admins, sessions, bootstrap_codes, audit_events,
		mfa_enrollments, recovery_codes, step_up_grants, member_invitations,
		login_challenges, external_identity_binding, oauth_identity_binding, oidc_transactions,
		applications, environments, secrets, secret_versions, file_bindings,
		drafts, draft_selections, bundle_revisions, revision_files,
		nodes, node_groups, group_members, assignments, enrollment_tokens
		, node_convergence
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}
