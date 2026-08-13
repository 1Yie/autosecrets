package database

import (
	"context"
	"errors"
	"strings"
	"time"

	"autosecrets.dev/core/internal/database/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound   = errors.New("store: not found")
	ErrDuplicate  = errors.New("store: duplicate")
	ErrConflict   = errors.New("store: version conflict")
	ErrBadPayload = errors.New("store: bad payload")
	ErrNoSecrets  = errors.New("store: no secrets")
)

func mapNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// --- Applications and Environments ----------------------------------------

type Application struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) ListApplications(ctx context.Context) ([]Application, error) {
	rows, err := s.q.ListApplications(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Application, len(rows))
	for i, r := range rows {
		out[i] = Application(r)
	}
	return out, nil
}

func (s *Store) CreateApplication(ctx context.Context, id, name string) error {
	err := s.q.CreateApplication(ctx, gen.CreateApplicationParams{ID: id, Name: name})
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}

func (s *Store) GetApplication(ctx context.Context, id string) (*Application, error) {
	r, err := s.q.GetApplication(ctx, id)
	if err != nil {
		return nil, mapNoRows(err)
	}
	a := Application(r)
	return &a, nil
}

type Environment struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ApplicationID string `json:"application_id"`
	Protection    string `json:"protection"`
}

func (s *Store) ListEnvironments(ctx context.Context, appID string) ([]Environment, error) {
	rows, err := s.q.ListEnvironments(ctx, appID)
	if err != nil {
		return nil, err
	}
	out := make([]Environment, len(rows))
	for i, r := range rows {
		out[i] = Environment{ID: r.ID, Name: r.Name, ApplicationID: r.ApplicationID, Protection: r.ProtectionLevel}
	}
	return out, nil
}

func (s *Store) CreateEnvironment(ctx context.Context, id, appID, name, protection string) error {
	err := s.q.CreateEnvironment(ctx, gen.CreateEnvironmentParams{
		ID: id, ApplicationID: appID, Name: name, ProtectionLevel: protection,
	})
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}

func (s *Store) GetEnvironment(ctx context.Context, envID, appID string) (*Environment, error) {
	r, err := s.q.GetEnvironment(ctx, gen.GetEnvironmentParams{ID: envID, ApplicationID: appID})
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &Environment{ID: r.ID, Name: r.Name, ApplicationID: r.ApplicationID, Protection: r.ProtectionLevel}, nil
}

// --- Secrets, versions, bindings, drafts, revisions -----------------------

type SecretListRow struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	UID             int64  `json:"uid"`
	GID             int64  `json:"gid"`
	Mode            int64  `json:"mode"`
	LatestVersion   int64  `json:"latest_version"`
	SelectedVersion int64  `json:"selected_version"`
}

func (s *Store) ListSecrets(ctx context.Context, appID, envID string) ([]SecretListRow, error) {
	rows, err := s.q.ListSecrets(ctx, gen.ListSecretsParams{ApplicationID: appID, EnvironmentID: envID})
	if err != nil {
		return nil, err
	}
	out := make([]SecretListRow, len(rows))
	for i, r := range rows {
		out[i] = SecretListRow{
			ID: r.ID, Name: r.Name, Path: r.Path, UID: r.Uid, GID: r.Gid, Mode: r.Mode,
			LatestVersion: r.Column7, SelectedVersion: r.VersionSeq,
		}
	}
	return out, nil
}

// CreateSecretWithValue creates a Secret, its first encrypted Secret Version
// (seq 1), the default File Binding (filename equals the Secret name, 0400),
// and the Draft selection in one transaction.
func (s *Store) CreateSecretWithValue(ctx context.Context, secretID, versionID, appID, envID, name string,
	wrappedKey, nonces, ciphertext []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	if err := q.InsertSecret(ctx, gen.InsertSecretParams{
		ID: secretID, ApplicationID: appID, EnvironmentID: envID, Name: name,
	}); err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return err
	}
	if err := q.InsertSecretVersionFirst(ctx, gen.InsertSecretVersionFirstParams{
		ID: versionID, SecretID: secretID, WrappedKey: wrappedKey, Nonce: nonces, Ciphertext: ciphertext,
	}); err != nil {
		return err
	}
	if err := q.InsertDefaultFileBinding(ctx, gen.InsertDefaultFileBindingParams{
		SecretID: secretID, Path: name,
	}); err != nil {
		return err
	}
	if err := ensureDraftAndBump(ctx, q, appID, envID); err != nil {
		return err
	}
	if err := q.InsertDraftSelectionForNewSecret(ctx, gen.InsertDraftSelectionForNewSecretParams{
		SecretID: secretID, ApplicationID: appID, EnvironmentID: envID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AddSecretVersion appends an immutable Secret Version and points the Draft
// selection at it. Returns the new seq and the bumped Draft version.
func (s *Store) AddSecretVersion(ctx context.Context, versionID, secretID string,
	wrappedKey, nonces, ciphertext []byte) (int64, int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	appEnv, err := q.SelectSecretAppEnv(ctx, secretID)
	if err != nil {
		return 0, 0, mapNoRows(err)
	}
	seq, err := q.InsertSecretVersionNext(ctx, gen.InsertSecretVersionNextParams{
		ID: versionID, SecretID: secretID, WrappedKey: wrappedKey, Nonce: nonces, Ciphertext: ciphertext,
	})
	if err != nil {
		return 0, 0, err
	}
	if err := q.UpdateDraftSelectionVersion(ctx, gen.UpdateDraftSelectionVersionParams{
		VersionSeq: seq, ApplicationID: appEnv.ApplicationID, EnvironmentID: appEnv.EnvironmentID, SecretID: secretID,
	}); err != nil {
		return 0, 0, err
	}
	draftVersion, err := bumpDraft(ctx, q, appEnv.ApplicationID, appEnv.EnvironmentID)
	if err != nil {
		return 0, 0, err
	}
	return seq, draftVersion, tx.Commit(ctx)
}

// UpdateBinding replaces a Secret's File Binding and bumps the Draft version.
func (s *Store) UpdateBinding(ctx context.Context, secretID, path string, uid, gid, mode int64) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	appEnv, err := q.SelectSecretAppEnv(ctx, secretID)
	if err != nil {
		return 0, mapNoRows(err)
	}
	if err := q.UpdateFileBinding(ctx, gen.UpdateFileBindingParams{
		Path: path, Uid: uid, Gid: gid, Mode: mode, SecretID: secretID,
	}); err != nil {
		return 0, err
	}
	draftVersion, err := bumpDraft(ctx, q, appEnv.ApplicationID, appEnv.EnvironmentID)
	if err != nil {
		return 0, err
	}
	return draftVersion, tx.Commit(ctx)
}

type DraftSelection struct {
	SecretID   string `json:"secret_id"`
	Name       string `json:"name"`
	VersionSeq int64  `json:"version_seq"`
	Path       string `json:"path"`
	UID        int64  `json:"uid"`
	GID        int64  `json:"gid"`
	Mode       int64  `json:"mode"`
}

type Draft struct {
	Version    int64            `json:"version"`
	Selections []DraftSelection `json:"selections"`
}

func (s *Store) GetDraft(ctx context.Context, appID, envID string) (*Draft, error) {
	draft, err := s.q.SelectDraftIDVersion(ctx, gen.SelectDraftIDVersionParams{ApplicationID: appID, EnvironmentID: envID})
	if err != nil {
		return nil, mapNoRows(err)
	}
	rows, err := s.q.ListDraftSelections(ctx, gen.ListDraftSelectionsParams{ApplicationID: appID, EnvironmentID: envID})
	if err != nil {
		return nil, err
	}
	d := &Draft{Version: draft.Version, Selections: []DraftSelection{}}
	for _, r := range rows {
		d.Selections = append(d.Selections, DraftSelection{
			SecretID: r.ID, Name: r.Name, VersionSeq: r.VersionSeq,
			Path: r.Path, UID: r.Uid, GID: r.Gid, Mode: r.Mode,
		})
	}
	return d, nil
}

// UpdateDraftSelections applies a new secret→version map when expected matches
// the current Draft version (optimistic concurrency).
func (s *Store) UpdateDraftSelections(ctx context.Context, appID, envID string,
	expected int64, selections map[string]int64) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	draftID, err := q.SelectDraftIDForUpdate(ctx, gen.SelectDraftIDForUpdateParams{ApplicationID: appID, EnvironmentID: envID})
	if err != nil {
		return 0, mapNoRows(err)
	}
	version, err := q.SelectDraftVersion(ctx, draftID)
	if err != nil {
		return 0, err
	}
	if version != expected {
		return 0, ErrConflict
	}
	for secretID, seq := range selections {
		exists, err := q.SelectDraftSelectionExists(ctx, gen.SelectDraftSelectionExistsParams{
			ID: draftID, SecretID: secretID, Seq: seq,
		})
		if err != nil {
			return 0, err
		}
		if !exists {
			return 0, ErrBadPayload
		}
	}
	for secretID, seq := range selections {
		if err := q.UpsertDraftSelection(ctx, gen.UpsertDraftSelectionParams{
			DraftID: draftID, SecretID: secretID, VersionSeq: seq,
		}); err != nil {
			return 0, err
		}
	}
	newVersion, err := bumpDraftTx(ctx, q, draftID)
	if err != nil {
		return 0, err
	}
	return newVersion, tx.Commit(ctx)
}

type Revision struct {
	ID              string          `json:"id"`
	DraftVersion    int64           `json:"draft_version"`
	FileCount       int64           `json:"file_count"`
	CreatedBy       string          `json:"created_by"`
	CreatedAt       string          `json:"created_at"`
	OperationReason OperationReason `json:"operation_reason"`
}

// OperationReason records why Desired State changed (ADR-0020). The
// explanation is 10-500 characters; the external reference is optional.
type OperationReason struct {
	Category    string `json:"category"`
	Explanation string `json:"explanation"`
	ExternalRef string `json:"external_ref,omitempty"`
}

// Publish freezes the current Draft selection and File Bindings into an
// immutable Bundle Revision in one transaction.
func (s *Store) Publish(ctx context.Context, revisionID, appID, envID, actor string, reason OperationReason) (*Revision, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	draft, err := q.SelectDraftIDVersion(ctx, gen.SelectDraftIDVersionParams{ApplicationID: appID, EnvironmentID: envID})
	if err != nil {
		return nil, mapNoRows(err)
	}
	n, err := q.CountDraftSelections(ctx, draft.ID)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNoSecrets
	}
	if err := q.InsertBundleRevision(ctx, gen.InsertBundleRevisionParams{
		ID: revisionID, ApplicationID: appID, EnvironmentID: envID, DraftVersion: draft.Version,
		CreatedBy: actor, OperationReasonCategory: reason.Category,
		OperationReasonExplanation: reason.Explanation, OperationReasonExternalRef: reason.ExternalRef,
	}); err != nil {
		return nil, err
	}
	if err := q.InsertRevisionFilesFromDraft(ctx, gen.InsertRevisionFilesFromDraftParams{
		RevisionID: revisionID, DraftID: draft.ID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Revision{ID: revisionID, DraftVersion: draft.Version, FileCount: n,
		CreatedBy: actor, CreatedAt: time.Now().UTC().Format(time.RFC3339),
		OperationReason: reason}, nil
}

func (s *Store) ListRevisions(ctx context.Context, appID, envID string) ([]Revision, error) {
	rows, err := s.q.ListRevisions(ctx, gen.ListRevisionsParams{ApplicationID: appID, EnvironmentID: envID})
	if err != nil {
		return nil, err
	}
	out := make([]Revision, len(rows))
	for i, r := range rows {
		out[i] = Revision{
			ID: r.ID, DraftVersion: r.DraftVersion, CreatedBy: r.CreatedBy,
			CreatedAt: r.ToChar, FileCount: r.Count,
			OperationReason: OperationReason{
				Category: r.OperationReasonCategory, Explanation: r.OperationReasonExplanation,
				ExternalRef: r.OperationReasonExternalRef,
			},
		}
	}
	return out, nil
}

func (s *Store) ListAllRevisions(ctx context.Context, limit int) ([]Revision, error) {
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	rows, err := s.q.ListAllRevisions(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]Revision, len(rows))
	for i, r := range rows {
		out[i] = Revision{
			ID: r.ID, DraftVersion: r.DraftVersion, CreatedBy: r.CreatedBy,
			CreatedAt: r.ToChar, FileCount: r.Count,
			OperationReason: OperationReason{
				Category: r.OperationReasonCategory, Explanation: r.OperationReasonExplanation,
				ExternalRef: r.OperationReasonExternalRef,
			},
		}
	}
	return out, nil
}

// Rollback copies an earlier Bundle Revision's snapshot into a new immutable
// Bundle Revision (ADR-0019). Historical revisions are never mutated.
func (s *Store) Rollback(ctx context.Context, newRevisionID, appID, envID, fromRevisionID, actor string, reason OperationReason) (*Revision, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	draftVersion, err := q.SelectRevisionDraftVersion(ctx, gen.SelectRevisionDraftVersionParams{
		ID: fromRevisionID, ApplicationID: appID, EnvironmentID: envID,
	})
	if err != nil {
		return nil, mapNoRows(err)
	}
	fileCount, err := q.CountRevisionFiles(ctx, fromRevisionID)
	if err != nil {
		return nil, err
	}
	if fileCount == 0 {
		return nil, ErrNoSecrets
	}
	if err := q.InsertBundleRevision(ctx, gen.InsertBundleRevisionParams{
		ID: newRevisionID, ApplicationID: appID, EnvironmentID: envID, DraftVersion: draftVersion,
		CreatedBy: actor, OperationReasonCategory: reason.Category,
		OperationReasonExplanation: reason.Explanation, OperationReasonExternalRef: reason.ExternalRef,
	}); err != nil {
		return nil, err
	}
	if err := q.InsertRevisionFilesCopy(ctx, gen.InsertRevisionFilesCopyParams{
		RevisionID: newRevisionID, RevisionID_2: fromRevisionID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Revision{ID: newRevisionID, DraftVersion: draftVersion, FileCount: fileCount,
		CreatedBy: actor, CreatedAt: time.Now().UTC().Format(time.RFC3339),
		OperationReason: reason}, nil
}

func ensureDraftAndBump(ctx context.Context, q *gen.Queries, appID, envID string) error {
	if err := q.InsertDraft(ctx, gen.InsertDraftParams{ApplicationID: appID, EnvironmentID: envID}); err != nil {
		return err
	}
	_, err := bumpDraft(ctx, q, appID, envID)
	return err
}

func bumpDraft(ctx context.Context, q *gen.Queries, appID, envID string) (int64, error) {
	draftID, err := q.SelectDraftID(ctx, gen.SelectDraftIDParams{ApplicationID: appID, EnvironmentID: envID})
	if err != nil {
		return 0, err
	}
	return bumpDraftTx(ctx, q, draftID)
}

func bumpDraftTx(ctx context.Context, q *gen.Queries, draftID string) (int64, error) {
	return q.BumpDraft(ctx, draftID)
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}

// --- Node Groups, Assignments, Nodes --------------------------------------

type NodeGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	MemberIDs []string  `json:"member_ids"`
}

func (s *Store) ListNodeGroups(ctx context.Context) ([]NodeGroup, error) {
	rows, err := s.q.ListNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]NodeGroup, len(rows))
	for i, r := range rows {
		out[i] = NodeGroup{ID: r.ID, Name: r.Name, MemberIDs: r.MemberIds}
	}
	return out, nil
}

func (s *Store) CreateNodeGroup(ctx context.Context, id, name string) error {
	err := s.q.CreateNodeGroup(ctx, gen.CreateNodeGroupParams{ID: id, Name: name})
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}

func (s *Store) AddGroupMember(ctx context.Context, groupID, nodeID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	if err := q.InsertGroupMember(ctx, gen.InsertGroupMemberParams{GroupID: groupID, NodeID: nodeID}); err != nil {
		if strings.Contains(err.Error(), "violates foreign key constraint") {
			return ErrNotFound
		}
		return err
	}
	ambiguous, err := q.SelectGroupMemberConflict(ctx, gen.SelectGroupMemberConflictParams{GroupID: groupID, NodeID: nodeID})
	if err != nil {
		return err
	}
	if ambiguous {
		return ErrConflict
	}
	return tx.Commit(ctx)
}

func (s *Store) RemoveGroupMember(ctx context.Context, groupID, nodeID string) error {
	return s.q.RemoveGroupMember(ctx, gen.RemoveGroupMemberParams{GroupID: groupID, NodeID: nodeID})
}

type Assignment struct {
	ID            string `json:"id"`
	GroupID       string `json:"group_id"`
	GroupName     string `json:"group_name"`
	ApplicationID string `json:"application_id"`
	EnvironmentID string `json:"environment_id"`
	RevisionID    string `json:"revision_id"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

func (s *Store) ListAssignments(ctx context.Context) ([]Assignment, error) {
	rows, err := s.q.ListAssignments(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Assignment, len(rows))
	for i, r := range rows {
		out[i] = Assignment{
			ID: r.ID, GroupID: r.GroupID, GroupName: r.Name,
			ApplicationID: r.ApplicationID, EnvironmentID: r.EnvironmentID,
			RevisionID: r.RevisionID, Status: r.Status, CreatedAt: r.ToChar,
		}
	}
	return out, nil
}

func (s *Store) CreateAssignment(ctx context.Context, id, groupID, appID, envID string) (*Assignment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	groupName, err := q.SelectNodeGroupName(ctx, groupID)
	if err != nil {
		return nil, mapNoRows(err)
	}
	envExists, err := q.SelectEnvironmentExists(ctx, gen.SelectEnvironmentExistsParams{ID: envID, ApplicationID: appID})
	if err != nil {
		return nil, err
	}
	if !envExists {
		return nil, ErrNotFound
	}
	revisionID, err := q.SelectLatestRevision(ctx, gen.SelectLatestRevisionParams{ApplicationID: appID, EnvironmentID: envID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBadPayload
		}
		return nil, err
	}
	ambiguous, err := q.SelectAssignmentConflict(ctx, gen.SelectAssignmentConflictParams{
		ApplicationID: appID, EnvironmentID: envID, GroupID: groupID,
	})
	if err != nil {
		return nil, err
	}
	if ambiguous {
		return nil, ErrConflict
	}
	createdAt, err := q.InsertAssignment(ctx, gen.InsertAssignmentParams{
		ID: id, GroupID: groupID, ApplicationID: appID, EnvironmentID: envID, RevisionID: revisionID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Assignment{ID: id, GroupID: groupID, GroupName: groupName,
		ApplicationID: appID, EnvironmentID: envID, RevisionID: revisionID,
		Status: "active", CreatedAt: createdAt}, nil
}

func (s *Store) AdvanceDesiredRevision(ctx context.Context, appID, envID, revisionID string) error {
	return s.q.AdvanceDesiredRevision(ctx, gen.AdvanceDesiredRevisionParams{
		ApplicationID: appID, EnvironmentID: envID, RevisionID: revisionID,
	})
}

type Node struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Serial           string     `json:"serial"`
	AgePubkey        string     `json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
	LastSeenAt       *time.Time `json:"last_seen_at"`
	DesiredETag      string     `json:"desired_etag"`
	ObservedRevision string     `json:"observed_revision"`
	LastResult       string     `json:"last_result"`
}

func (s *Store) ListNode(ctx context.Context) ([]Node, error) {
	rows, err := s.q.ListNode(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Node, len(rows))
	for i, r := range rows {
		out[i] = Node{
			ID: r.ID, Name: r.Name, Serial: r.Serial, CreatedAt: r.CreatedAt,
			LastSeenAt: tsPtr(r.LastSeenAt), DesiredETag: r.DesiredEtag,
			ObservedRevision: r.ObservedRevision, LastResult: r.LastResult,
		}
	}
	return out, nil
}

func (s *Store) NodeBySerial(ctx context.Context, serial string) (*Node, error) {
	r, err := s.q.NodeBySerial(ctx, serial)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &Node{
		ID: r.ID, Name: r.Name, Serial: r.Serial, AgePubkey: r.AgePubkey, CreatedAt: r.CreatedAt,
		LastSeenAt: tsPtr(r.LastSeenAt), DesiredETag: r.DesiredEtag,
		ObservedRevision: r.ObservedRevision, LastResult: r.LastResult,
	}, nil
}

func (s *Store) TouchNode(ctx context.Context, nodeID string, observedRevision, lastResult string, at time.Time) error {
	return s.q.TouchNode(ctx, gen.TouchNodeParams{
		ID: nodeID, ObservedRevision: observedRevision, LastResult: lastResult, LastSeenAt: pgTS(&at),
	})
}

func (s *Store) SetNodeDesired(ctx context.Context, nodeID, etag string) error {
	return s.q.SetNodeDesired(ctx, gen.SetNodeDesiredParams{ID: nodeID, DesiredEtag: etag})
}

// --- Enrollment tokens -----------------------------------------------------

type EnrollmentToken struct {
	TokenHash string    `json:"-"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Store) CreateEnrollmentToken(ctx context.Context, tokenHash, name string, expiresAt time.Time) error {
	return s.q.CreateEnrollmentToken(ctx, gen.CreateEnrollmentTokenParams{
		TokenHash: tokenHash, Name: name, ExpiresAt: expiresAt,
	})
}

func (s *Store) ConsumeEnrollmentToken(ctx context.Context, tokenHash string, now time.Time) (*EnrollmentToken, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	r, err := q.SelectEnrollmentTokenForUpdate(ctx, tokenHash)
	if err != nil {
		return nil, mapNoRows(err)
	}
	t := EnrollmentToken{TokenHash: r.TokenHash, Name: r.Name, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt}
	if !t.ExpiresAt.After(now) {
		return nil, ErrConflict
	}
	used, err := q.SelectEnrollmentTokenUsedAt(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if used.Valid {
		return nil, ErrConflict
	}
	if err := q.MarkEnrollmentTokenUsed(ctx, gen.MarkEnrollmentTokenUsedParams{
		TokenHash: tokenHash, UsedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		return nil, err
	}
	return &t, tx.Commit(ctx)
}

func (s *Store) RegisterNode(ctx context.Context, id, name, serial, agePubkey, certPEM string, certExpiresAt time.Time) error {
	return s.q.RegisterNode(ctx, gen.RegisterNodeParams{
		ID: id, Name: name, Serial: serial, AgePubkey: agePubkey, CertPem: certPEM, CertExpiresAt: certExpiresAt,
	})
}

// --- Desired State (delivery) ----------------------------------------------

type RevisionFile struct {
	RevisionID string
	SecretID   string
	Path       string
	UID        int64
	GID        int64
	Mode       int64
	VersionSeq int64
}

func (s *Store) AssignedRevisions(ctx context.Context, nodeID string) ([]string, error) {
	return s.q.AssignedRevisions(ctx, nodeID)
}

func (s *Store) RevisionFiles(ctx context.Context, revisionID string) ([]RevisionFile, error) {
	rows, err := s.q.RevisionFiles(ctx, revisionID)
	if err != nil {
		return nil, err
	}
	out := make([]RevisionFile, len(rows))
	for i, r := range rows {
		out[i] = RevisionFile{
			RevisionID: r.RevisionID, SecretID: r.SecretID, Path: r.Path,
			UID: r.Uid, GID: r.Gid, Mode: r.Mode, VersionSeq: r.VersionSeq,
		}
	}
	return out, nil
}

func (s *Store) RevisionAppEnv(ctx context.Context, revisionID string) (appID, envID string, err error) {
	r, err := s.q.RevisionAppEnv(ctx, revisionID)
	return r.ApplicationID, r.EnvironmentID, mapNoRows(err)
}

func (s *Store) SecretVersionBlob(ctx context.Context, secretID string, seq int64) (wrappedKey, nonces, ciphertext []byte, err error) {
	r, err := s.q.SecretVersionBlob(ctx, gen.SecretVersionBlobParams{SecretID: secretID, Seq: seq})
	return r.WrappedKey, r.Nonce, r.Ciphertext, mapNoRows(err)
}

func (s *Store) SecretAppEnv(ctx context.Context, secretID string) (appID, envID string, err error) {
	r, err := s.q.SecretAppEnv(ctx, secretID)
	return r.ApplicationID, r.EnvironmentID, mapNoRows(err)
}
