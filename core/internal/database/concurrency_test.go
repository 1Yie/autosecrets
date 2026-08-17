package database_test

// Direct store-level tests for transaction invariants that are hard to
// observe through the HTTP seam: optimistic-lock conflicts and single-use
// consumption under concurrency. Package store has no other direct tests by
// design (the implementation plan mandates testing through module
// interfaces); these exist only where the invariant is inherently racy.

import (
	"context"
	"sync"
	"testing"
	"time"

	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/testutil"
	"github.com/google/uuid"
)

func newStore(t *testing.T) *database.Store {
	t.Helper()
	st := testutil.Connect(t)
	testutil.Truncate(t, st)
	return st
}

// seedSecret creates an Application, Environment, and one Secret with its
// first encrypted version, File Binding, and Draft row (mirrors
// CreateSecretWithValue). The ciphertext bytes are arbitrary; the store
// never interprets them.
func seedSecret(t *testing.T, st *database.Store) (appID, envID, secretID string) {
	t.Helper()
	ctx := context.Background()
	appID, envID, secretID = uuid.NewString(), uuid.NewString(), uuid.NewString()
	if err := st.CreateApplication(ctx, appID, "payments"); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateEnvironment(ctx, envID, appID, "production"); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSecretWithValue(ctx, secretID, uuid.NewString(), appID, envID,
		"db_token", []byte("wrapped"), []byte("nonce"), []byte("ct")); err != nil {
		t.Fatal(err)
	}
	return appID, envID, secretID
}

// TestDraftSelectionsConcurrentConflict proves the optimistic-concurrency
// invariant: two concurrent updates with the same expected Draft version
// produce exactly one success and one ErrConflict, never a silent overwrite.
func TestDraftSelectionsConcurrentConflict(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	appID, envID, secretID := seedSecret(t, st)

	const expected = 1
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := st.UpdateDraftSelections(ctx, appID, envID, expected,
				map[string]int64{secretID: 1})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	var successes, conflicts int
	for err := range results {
		switch err {
		case nil:
			successes++
		case database.ErrConflict:
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("want 1 success and 1 conflict, got %d/%d", successes, conflicts)
	}
}

// TestEnrollmentTokenConcurrentSingleUse proves an Enrollment Token is
// consumed exactly once even when several Agents race with it.
func TestEnrollmentTokenConcurrentSingleUse(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	tokenHash := "tok-hash-1"
	if err := st.CreateEnrollmentToken(ctx, tokenHash, "node-1",
		time.Now().Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}

	const racers = 10
	results := make(chan error, racers)
	var wg sync.WaitGroup
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := st.ConsumeEnrollmentToken(ctx, tokenHash, time.Now())
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	var successes, conflicts int
	for err := range results {
		switch err {
		case nil:
			successes++
		case database.ErrConflict:
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successes != 1 || conflicts != racers-1 {
		t.Fatalf("want 1 success and %d conflicts, got %d/%d", racers-1, successes, conflicts)
	}
}

// TestEnrollmentTokenExpired proves an expired Token cannot be consumed.
func TestEnrollmentTokenExpired(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	tokenHash := "tok-hash-expired"
	if err := st.CreateEnrollmentToken(ctx, tokenHash, "node-x",
		time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ConsumeEnrollmentToken(ctx, tokenHash, time.Now()); err != database.ErrConflict {
		t.Fatalf("expired token must conflict, got %v", err)
	}
}

// TestPublishEmptyDraft proves an empty Draft cannot be frozen into a
// Bundle Revision. The selection is removed directly because
// UpdateDraftSelections only upserts the given keys and the management API
// forbids empty selection maps; the invariant under test is Publish's.
func TestPublishEmptyDraft(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	appID, envID, _ := seedSecret(t, st)
	if _, err := st.Exec(ctx, `DELETE FROM draft_selections WHERE draft_id IN (
		SELECT id FROM drafts WHERE application_id = $1 AND environment_id = $2)`,
		appID, envID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Publish(ctx, uuid.NewString(), appID, envID, "admin", database.OperationReason{
		Category: "maintenance", Explanation: "empty draft must fail",
	}); err != database.ErrNoSecrets {
		t.Fatalf("empty draft publish must fail with ErrNoSecrets, got %v", err)
	}
}
