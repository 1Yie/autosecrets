package database_test

// Full-lifecycle store test: walks every persistence method once so the
// package has direct coverage of CRUD paths in addition to the racy
// invariants in concurrency_test.go. All ciphertext here is opaque bytes;
// encryption is app-layer's job.

import (
	"context"
	"testing"
	"time"

	"autosecrets.dev/core/internal/database"
	"github.com/google/uuid"
)

func TestStoreLifecycle(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()

	// Applications.
	if err := st.CreateApplication(ctx, uuid.NewString(), "payments"); err != nil {
		t.Fatal(err)
	}
	appID := uuid.NewString()
	if err := st.CreateApplication(ctx, appID, "inventory"); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateApplication(ctx, uuid.NewString(), "payments"); err != database.ErrDuplicate {
		t.Fatalf("duplicate application name must be ErrDuplicate, got %v", err)
	}
	apps, err := st.ListApplications(ctx)
	if err != nil || len(apps) != 2 {
		t.Fatalf("list applications: %v (%d)", err, len(apps))
	}
	app, err := st.GetApplication(ctx, appID)
	if err != nil || app.Name != "inventory" {
		t.Fatalf("get application: %v %+v", err, app)
	}
	if _, err := st.GetApplication(ctx, uuid.NewString()); err != database.ErrNotFound {
		t.Fatalf("missing application must be ErrNotFound, got %v", err)
	}

	// Environments.
	if err := st.CreateEnvironment(ctx, uuid.NewString(), appID, "production", "standard"); err != nil {
		t.Fatal(err)
	}
	envID := uuid.NewString()
	if err := st.CreateEnvironment(ctx, envID, appID, "staging", "standard"); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateEnvironment(ctx, uuid.NewString(), appID, "production", "standard"); err != database.ErrDuplicate {
		t.Fatalf("duplicate environment must be ErrDuplicate, got %v", err)
	}
	envs, err := st.ListEnvironments(ctx, appID)
	if err != nil || len(envs) != 2 {
		t.Fatalf("list environments: %v (%d)", err, len(envs))
	}
	if _, err := st.GetEnvironment(ctx, envID, appID); err != nil {
		t.Fatalf("get environment: %v", err)
	}
	if _, err := st.GetEnvironment(ctx, envID, uuid.NewString()); err != database.ErrNotFound {
		t.Fatalf("environment under wrong application must be ErrNotFound, got %v", err)
	}

	// Secrets: create, list, rotate, binding, draft.
	secretID := uuid.NewString()
	if err := st.CreateSecretWithValue(ctx, secretID, uuid.NewString(), appID, envID,
		"db_pass", []byte("w1"), []byte("n1"), []byte("c1")); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSecretWithValue(ctx, uuid.NewString(), uuid.NewString(), appID, envID,
		"db_pass", []byte("w1"), []byte("n1"), []byte("c1")); err != database.ErrDuplicate {
		t.Fatalf("duplicate secret must be ErrDuplicate, got %v", err)
	}
	rows, err := st.ListSecrets(ctx, appID, envID)
	if err != nil || len(rows) != 1 || rows[0].Name != "db_pass" || rows[0].LatestVersion != 1 {
		t.Fatalf("list secrets: %v %+v", err, rows)
	}
	seq, draftV, err := st.AddSecretVersion(ctx, uuid.NewString(), secretID,
		[]byte("w2"), []byte("n2"), []byte("c2"))
	if err != nil || seq != 2 || draftV < 1 {
		t.Fatalf("add version: %v seq=%d draft=%d", err, seq, draftV)
	}
	if _, _, err := st.AddSecretVersion(ctx, uuid.NewString(), uuid.NewString(),
		[]byte("w"), []byte("n"), []byte("c")); err != database.ErrNotFound {
		t.Fatalf("version on missing secret must be ErrNotFound, got %v", err)
	}
	draftV2, err := st.UpdateBinding(ctx, secretID, "app/db", 1000, 1000, 0o600)
	if err != nil || draftV2 <= draftV {
		t.Fatalf("update binding: %v draft=%d", err, draftV2)
	}
	draft, err := st.GetDraft(ctx, appID, envID)
	if err != nil || draft.Version != draftV2 || len(draft.Selections) != 1 ||
		draft.Selections[0].Path != "app/db" {
		t.Fatalf("get draft: %v %+v", err, draft)
	}
	newDraftV, err := st.UpdateDraftSelections(ctx, appID, envID, draftV2,
		map[string]int64{secretID: 1})
	if err != nil || newDraftV <= draftV2 {
		t.Fatalf("update draft selections: %v v=%d", err, newDraftV)
	}
	if _, err := st.UpdateDraftSelections(ctx, appID, envID, draftV2,
		map[string]int64{secretID: 1}); err != database.ErrConflict {
		t.Fatalf("stale draft update must be ErrConflict, got %v", err)
	}
	if _, err := st.UpdateDraftSelections(ctx, appID, envID, newDraftV,
		map[string]int64{secretID: 99}); err != database.ErrBadPayload {
		t.Fatalf("unknown version selection must be ErrBadPayload, got %v", err)
	}

	// Publish and revisions.
	rev, err := st.Publish(ctx, uuid.NewString(), appID, envID, "admin", database.OperationReason{
		Category: "maintenance", Explanation: "lifecycle test publish",
	})
	if err != nil || rev.FileCount != 1 {
		t.Fatalf("publish: %v %+v", err, rev)
	}
	revs, err := st.ListRevisions(ctx, appID, envID)
	if err != nil || len(revs) != 1 || revs[0].CreatedBy != "admin" {
		t.Fatalf("list revisions: %v %+v", err, revs)
	}

	// Node groups, members, assignments.
	groupID := uuid.NewString()
	if err := st.CreateNodeGroup(ctx, groupID, "web-tier"); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateNodeGroup(ctx, uuid.NewString(), "web-tier"); err != database.ErrDuplicate {
		t.Fatalf("duplicate group must be ErrDuplicate, got %v", err)
	}
	if err := st.AddGroupMember(ctx, groupID, uuid.NewString()); err != database.ErrNotFound {
		t.Fatalf("member of missing node must be ErrNotFound, got %v", err)
	}
	groups, err := st.ListNodeGroups(ctx)
	if err != nil || len(groups) != 1 {
		t.Fatalf("list groups: %v %+v", err, groups)
	}
	assignID := uuid.NewString()
	if err := st.SaveActivationPolicy(ctx, database.ActivationPolicy{
		EnvironmentID: envID, Action: "restart", Units: []string{"web.service"},
	}); err != nil {
		t.Fatal(err)
	}
	asg, err := st.CreateAssignment(ctx, assignID, groupID, appID, envID)
	if err != nil {
		t.Fatal(err)
	}
	if asg.RevisionID != rev.ID {
		t.Fatalf("assignment must follow the current desired revision, got %s want %s", asg.RevisionID, rev.ID)
	}
	if _, err := st.CreateAssignment(ctx, uuid.NewString(), groupID, appID, envID); err != database.ErrDuplicate {
		t.Fatalf("duplicate assignment must be ErrDuplicate, got %v", err)
	}
	if _, err := st.CreateAssignment(ctx, uuid.NewString(), uuid.NewString(), appID, envID); err != database.ErrNotFound {
		t.Fatalf("assignment to missing group must be ErrNotFound, got %v", err)
	}
	assigns, err := st.ListAssignments(ctx)
	if err != nil || len(assigns) != 1 || assigns[0].GroupName != "web-tier" {
		t.Fatalf("list assignments: %v %+v", err, assigns)
	}

	// Nodes: register, list, lookup, touch, desired state.
	nodeID := uuid.NewString()
	if err := st.RegisterNode(ctx, nodeID, "node-1", "serial-1",
		"age1", "cert1", time.Now().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := st.AddGroupMember(ctx, groupID, nodeID); err != nil {
		t.Fatal(err)
	}
	nodes, err := st.ListNode(ctx)
	if err != nil || len(nodes) != 1 || nodes[0].Serial != "serial-1" {
		t.Fatalf("list nodes: %v %+v", err, nodes)
	}
	node, err := st.NodeBySerial(ctx, "serial-1")
	if err != nil || node.ID != nodeID || node.AgePubkey != "age1" {
		t.Fatalf("node by serial: %v %+v", err, node)
	}
	if _, err := st.NodeBySerial(ctx, "nope"); err != database.ErrNotFound {
		t.Fatalf("missing node must be ErrNotFound, got %v", err)
	}
	if err := st.TouchNode(ctx, nodeID, rev.ID, "ok", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := st.SetNodeDesired(ctx, nodeID, `"etag"`); err != nil {
		t.Fatal(err)
	}
	assigned, err := st.AssignedRevisions(ctx, nodeID)
	if err != nil || len(assigned) != 1 || assigned[0] != rev.ID {
		t.Fatalf("assigned revisions: %v %+v", err, assigned)
	}
	files, err := st.RevisionFiles(ctx, rev.ID)
	if err != nil || len(files) != 1 || files[0].Path != "app/db" || files[0].VersionSeq != 1 {
		t.Fatalf("revision files: %v %+v", err, files)
	}
	appOf, envOf, err := st.RevisionAppEnv(ctx, rev.ID)
	if err != nil || appOf != appID || envOf != envID {
		t.Fatalf("revision app/env: %v %s %s", err, appOf, envOf)
	}
	wk, nn, ct, err := st.SecretVersionBlob(ctx, secretID, 1)
	if err != nil || string(wk) != "w1" || string(nn) != "n1" || string(ct) != "c1" {
		t.Fatalf("secret version blob: %v", err)
	}
	if err := st.RemoveGroupMember(ctx, groupID, nodeID); err != nil {
		t.Fatal(err)
	}
	assignedAfter, err := st.AssignedRevisions(ctx, nodeID)
	if err != nil || len(assignedAfter) != 0 {
		t.Fatalf("assigned after removal: %v %+v", err, assignedAfter)
	}

	// Identity: admins, bootstrap codes, sessions.
	if n, err := st.AdminCount(ctx); err != nil || n != 0 {
		t.Fatalf("admin count: %v %d", err, n)
	}
	adminID := uuid.NewString()
	if err := st.CreateAdmin(ctx, adminID, "alice", "hash"); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAdmin(ctx, uuid.NewString(), "alice", "hash"); err != database.ErrDuplicate {
		t.Fatalf("duplicate admin must be ErrDuplicate, got %v", err)
	}
	if err := st.ValidateSingleAdministrator(ctx); err != nil {
		t.Fatalf("single Administrator rejected: %v", err)
	}
	if err := st.CreateAdmin(ctx, uuid.NewString(), "bob", "hash"); err != nil {
		t.Fatal(err)
	}
	if err := st.ValidateSingleAdministrator(ctx); err == nil {
		t.Fatal("multiple human identities must be rejected at startup")
	}
	admin, err := st.AdminByUsername(ctx, "alice")
	if err != nil || admin.PasswordHash != "hash" {
		t.Fatalf("admin by username: %v %+v", err, admin)
	}
	if err := st.SaveBootstrapCode(ctx, "codehash", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.ConsumeBootstrapCode(ctx, "codehash", time.Now()); err != nil || !ok {
		t.Fatalf("consume bootstrap code: %v %v", err, ok)
	}
	if ok, err := st.ConsumeBootstrapCode(ctx, "codehash", time.Now()); err != nil || ok {
		t.Fatalf("bootstrap code reuse must fail: %v %v", err, ok)
	}
	sessionHash := "sesshash"
	if err := st.CreateSession(ctx, sessionHash, adminID, "csrftok", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	sess, err := st.SessionByID(ctx, sessionHash, time.Now())
	if err != nil || sess.AdminID != adminID || sess.CSRFToken != "csrftok" {
		t.Fatalf("session by id: %v %+v", err, sess)
	}
	if _, err := st.SessionByID(ctx, sessionHash, time.Now().Add(2*time.Hour)); err != database.ErrNotFound {
		t.Fatalf("expired session must be ErrNotFound, got %v", err)
	}
	if err := st.DeleteSession(ctx, sessionHash); err != nil {
		t.Fatal(err)
	}

	// Audit.
	if err := st.AppendAudit(ctx, nil, database.AuditEvent{
		Actor: "alice", Action: "secret.create", Resource: secretID,
		Result: "ok", CorrelationID: "corr-1",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := st.ListAudit(ctx, database.AuditFilter{Action: "secret.create", Limit: 10})
	if err != nil || len(events) != 1 || events[0].Actor != "alice" || events[0].CorrelationID != "corr-1" {
		t.Fatalf("list audit: %v %+v", err, events)
	}
	if events, err := st.ListAudit(ctx, database.AuditFilter{Limit: 5}); err != nil || len(events) != 1 {
		t.Fatalf("list audit all: %v %+v", err, events)
	}
}
