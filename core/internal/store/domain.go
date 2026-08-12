package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound   = errors.New("store: not found")
	ErrDuplicate  = errors.New("store: duplicate")
	ErrConflict   = errors.New("store: version conflict")
	ErrBadPayload = errors.New("store: bad payload")
	ErrNoSecrets  = errors.New("store: no secrets")
	ErrPolicy     = errors.New("store: activation policy required")
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
	rows, err := s.pool.Query(ctx, `SELECT id, name, created_at FROM applications ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Application{}
	for rows.Next() {
		var a Application
		if err := rows.Scan(&a.ID, &a.Name, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CreateApplication(ctx context.Context, id, name string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO applications (id, name) VALUES ($1, $2)`, id, name)
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}

func (s *Store) GetApplication(ctx context.Context, id string) (*Application, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, name, created_at FROM applications WHERE id = $1`, id)
	var a Application
	if err := row.Scan(&a.ID, &a.Name, &a.CreatedAt); err != nil {
		return nil, mapNoRows(err)
	}
	return &a, nil
}

type Environment struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ApplicationID string `json:"application_id"`
	Protection    string `json:"protection"`
}

func (s *Store) ListEnvironments(ctx context.Context, appID string) ([]Environment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, application_id, protection_level FROM environments WHERE application_id = $1 ORDER BY name`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Environment{}
	for rows.Next() {
		var e Environment
		if err := rows.Scan(&e.ID, &e.Name, &e.ApplicationID, &e.Protection); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) CreateEnvironment(ctx context.Context, id, appID, name, protection string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO environments (id, application_id, name, protection_level) VALUES ($1, $2, $3, $4)`,
		id, appID, name, protection)
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}

func (s *Store) GetEnvironment(ctx context.Context, envID, appID string) (*Environment, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, application_id, protection_level FROM environments WHERE id = $1 AND application_id = $2`, envID, appID)
	var e Environment
	if err := row.Scan(&e.ID, &e.Name, &e.ApplicationID, &e.Protection); err != nil {
		return nil, mapNoRows(err)
	}
	return &e, nil
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
	rows, err := s.pool.Query(ctx, `
		SELECT sc.id, sc.name,
		       COALESCE(fb.path, ''), COALESCE(fb.uid, 0), COALESCE(fb.gid, 0), COALESCE(fb.mode, 0),
		       COALESCE(latest.seq, 0), COALESCE(ds.version_seq, 0)
		FROM secrets sc
		LEFT JOIN file_bindings fb ON fb.secret_id = sc.id
		LEFT JOIN LATERAL (SELECT max(seq) AS seq FROM secret_versions sv WHERE sv.secret_id = sc.id) latest ON true
		LEFT JOIN drafts d ON d.application_id = sc.application_id AND d.environment_id = sc.environment_id
		LEFT JOIN draft_selections ds ON ds.draft_id = d.id AND ds.secret_id = sc.id
		WHERE sc.application_id = $1 AND sc.environment_id = $2 AND sc.retired_at IS NULL
		ORDER BY sc.name`, appID, envID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SecretListRow{}
	for rows.Next() {
		var r SecretListRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &r.UID, &r.GID, &r.Mode,
			&r.LatestVersion, &r.SelectedVersion); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
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
	if _, err := tx.Exec(ctx,
		`INSERT INTO secrets (id, application_id, environment_id, name) VALUES ($1, $2, $3, $4)`,
		secretID, appID, envID, name); err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO secret_versions (id, secret_id, seq, wrapped_key, nonce, ciphertext)
		 VALUES ($1, $2, 1, $3, $4, $5)`, versionID, secretID, wrappedKey, nonces, ciphertext); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO file_bindings (id, secret_id, path, uid, gid, mode)
		 VALUES (gen_random_uuid(), $1, $2, 0, 0, 0o400)`, secretID, name); err != nil {
		return err
	}
	if err := ensureDraftAndBump(ctx, tx, appID, envID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO draft_selections (draft_id, secret_id, version_seq)
		SELECT d.id, $1, 1 FROM drafts d
		WHERE d.application_id = $2 AND d.environment_id = $3
		ON CONFLICT (draft_id, secret_id) DO UPDATE SET version_seq = 1`,
		secretID, appID, envID); err != nil {
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
	var appID, envID string
	if err := tx.QueryRow(ctx,
		`SELECT application_id, environment_id FROM secrets WHERE id = $1`, secretID).Scan(&appID, &envID); err != nil {
		return 0, 0, mapNoRows(err)
	}
	var seq int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO secret_versions (id, secret_id, seq, wrapped_key, nonce, ciphertext)
		 SELECT $1, $2, COALESCE(max(seq), 0) + 1, $3, $4, $5 FROM secret_versions WHERE secret_id = $2
		 RETURNING seq`, versionID, secretID, wrappedKey, nonces, ciphertext).Scan(&seq); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE draft_selections ds SET version_seq = $1
		FROM drafts d
		WHERE ds.draft_id = d.id AND d.application_id = $2 AND d.environment_id = $3 AND ds.secret_id = $4`,
		seq, appID, envID, secretID); err != nil {
		return 0, 0, err
	}
	draftVersion, err := bumpDraft(ctx, tx, appID, envID)
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
	var appID, envID string
	if err := tx.QueryRow(ctx,
		`SELECT application_id, environment_id FROM secrets WHERE id = $1`, secretID).Scan(&appID, &envID); err != nil {
		return 0, mapNoRows(err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE file_bindings SET path = $1, uid = $2, gid = $3, mode = $4, updated_at = now()
		 WHERE secret_id = $5`, path, uid, gid, mode, secretID); err != nil {
		return 0, err
	}
	draftVersion, err := bumpDraft(ctx, tx, appID, envID)
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
	var version int64
	if err := s.pool.QueryRow(ctx,
		`SELECT version FROM drafts WHERE application_id = $1 AND environment_id = $2`,
		appID, envID).Scan(&version); err != nil {
		return nil, mapNoRows(err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT sc.id, sc.name, ds.version_seq,
		       COALESCE(fb.path, ''), COALESCE(fb.uid, 0), COALESCE(fb.gid, 0), COALESCE(fb.mode, 0)
		FROM draft_selections ds
		JOIN drafts d ON d.id = ds.draft_id
		JOIN secrets sc ON sc.id = ds.secret_id
		LEFT JOIN file_bindings fb ON fb.secret_id = sc.id
		WHERE d.application_id = $1 AND d.environment_id = $2
		ORDER BY sc.name`, appID, envID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	d := &Draft{Version: version, Selections: []DraftSelection{}}
	for rows.Next() {
		var sel DraftSelection
		if err := rows.Scan(&sel.SecretID, &sel.Name, &sel.VersionSeq,
			&sel.Path, &sel.UID, &sel.GID, &sel.Mode); err != nil {
			return nil, err
		}
		d.Selections = append(d.Selections, sel)
	}
	return d, rows.Err()
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
	var draftID string
	if err := tx.QueryRow(ctx,
		`SELECT id FROM drafts WHERE application_id = $1 AND environment_id = $2 FOR UPDATE`,
		appID, envID).Scan(&draftID); err != nil {
		return 0, mapNoRows(err)
	}
	var version int64
	if err := tx.QueryRow(ctx, `SELECT version FROM drafts WHERE id = $1`, draftID).Scan(&version); err != nil {
		return 0, err
	}
	if version != expected {
		return 0, ErrConflict
	}
	for secretID, seq := range selections {
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM secret_versions sv
			 JOIN secrets sc ON sc.id = sv.secret_id
			 JOIN drafts d ON d.application_id = sc.application_id AND d.environment_id = sc.environment_id
			 WHERE d.id = $1 AND sv.secret_id = $2 AND sv.seq = $3)`,
			draftID, secretID, seq).Scan(&exists); err != nil {
			return 0, err
		}
		if !exists {
			return 0, ErrBadPayload
		}
	}
	for secretID, seq := range selections {
		if _, err := tx.Exec(ctx, `
			INSERT INTO draft_selections (draft_id, secret_id, version_seq)
			VALUES ($1, $2, $3)
			ON CONFLICT (draft_id, secret_id) DO UPDATE SET version_seq = $3`,
			draftID, secretID, seq); err != nil {
			return 0, err
		}
	}
	newVersion, err := bumpDraftTx(ctx, tx, draftID)
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
	var draftID string
	var draftVersion int64
	if err := tx.QueryRow(ctx,
		`SELECT id, version FROM drafts WHERE application_id = $1 AND environment_id = $2`,
		appID, envID).Scan(&draftID, &draftVersion); err != nil {
		return nil, mapNoRows(err)
	}
	var n int64
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM draft_selections WHERE draft_id = $1`, draftID).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNoSecrets
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO bundle_revisions (id, application_id, environment_id, draft_version, created_by,
			operation_reason_category, operation_reason_explanation, operation_reason_external_ref)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		revisionID, appID, envID, draftVersion, actor,
		reason.Category, reason.Explanation, reason.ExternalRef); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO revision_files (revision_id, secret_id, path, uid, gid, mode, version_seq)
		SELECT $1, ds.secret_id, fb.path, fb.uid, fb.gid, fb.mode, ds.version_seq
		FROM draft_selections ds
		JOIN file_bindings fb ON fb.secret_id = ds.secret_id
		WHERE ds.draft_id = $2`, revisionID, draftID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Revision{ID: revisionID, DraftVersion: draftVersion, FileCount: n,
		CreatedBy: actor, CreatedAt: time.Now().UTC().Format(time.RFC3339),
		OperationReason: reason}, nil
}

func (s *Store) ListRevisions(ctx context.Context, appID, envID string) ([]Revision, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT br.id, br.draft_version, br.created_by,
		       br.operation_reason_category, br.operation_reason_explanation, br.operation_reason_external_ref,
		       to_char(br.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       (SELECT count(*) FROM revision_files rf WHERE rf.revision_id = br.id)
		FROM bundle_revisions br
		WHERE br.application_id = $1 AND br.environment_id = $2
		ORDER BY br.created_at DESC, br.id DESC`, appID, envID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Revision{}
	for rows.Next() {
		var r Revision
		if err := rows.Scan(&r.ID, &r.DraftVersion, &r.CreatedBy,
			&r.OperationReason.Category, &r.OperationReason.Explanation, &r.OperationReason.ExternalRef,
			&r.CreatedAt, &r.FileCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListAllRevisions returns the newest Bundle Revisions across every
// Application and Environment, for the Overview projection.
func (s *Store) ListAllRevisions(ctx context.Context, limit int) ([]Revision, error) {
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	rows, err := s.pool.Query(ctx, `
		SELECT br.id, br.draft_version, br.created_by,
		       br.operation_reason_category, br.operation_reason_explanation, br.operation_reason_external_ref,
		       to_char(br.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       (SELECT count(*) FROM revision_files rf WHERE rf.revision_id = br.id)
		FROM bundle_revisions br
		ORDER BY br.created_at DESC, br.id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Revision{}
	for rows.Next() {
		var revision Revision
		if err := rows.Scan(&revision.ID, &revision.DraftVersion, &revision.CreatedBy,
			&revision.OperationReason.Category, &revision.OperationReason.Explanation,
			&revision.OperationReason.ExternalRef, &revision.CreatedAt, &revision.FileCount); err != nil {
			return nil, err
		}
		out = append(out, revision)
	}
	return out, rows.Err()
}

// Rollback copies an earlier Bundle Revision's snapshot into a new immutable
// Bundle Revision (ADR-0019). Historical revisions are never mutated.
func (s *Store) Rollback(ctx context.Context, newRevisionID, appID, envID, fromRevisionID, actor string, reason OperationReason) (*Revision, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var draftVersion int64
	if err := tx.QueryRow(ctx,
		`SELECT draft_version FROM bundle_revisions
		 WHERE id = $1 AND application_id = $2 AND environment_id = $3`,
		fromRevisionID, appID, envID).Scan(&draftVersion); err != nil {
		return nil, mapNoRows(err)
	}
	var fileCount int64
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM revision_files WHERE revision_id = $1`, fromRevisionID).Scan(&fileCount); err != nil {
		return nil, err
	}
	if fileCount == 0 {
		return nil, ErrNoSecrets
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO bundle_revisions (id, application_id, environment_id, draft_version, created_by,
			operation_reason_category, operation_reason_explanation, operation_reason_external_ref)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		newRevisionID, appID, envID, draftVersion, actor,
		reason.Category, reason.Explanation, reason.ExternalRef); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO revision_files (revision_id, secret_id, path, uid, gid, mode, version_seq)
		SELECT $1, secret_id, path, uid, gid, mode, version_seq
		FROM revision_files WHERE revision_id = $2`,
		newRevisionID, fromRevisionID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Revision{ID: newRevisionID, DraftVersion: draftVersion, FileCount: fileCount,
		CreatedBy: actor, CreatedAt: time.Now().UTC().Format(time.RFC3339),
		OperationReason: reason}, nil
}

func ensureDraftAndBump(ctx context.Context, tx pgx.Tx, appID, envID string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO drafts (id, application_id, environment_id)
		VALUES (gen_random_uuid(), $1, $2)
		ON CONFLICT (application_id, environment_id) DO NOTHING`, appID, envID); err != nil {
		return err
	}
	_, err := bumpDraft(ctx, tx, appID, envID)
	return err
}

func bumpDraft(ctx context.Context, tx pgx.Tx, appID, envID string) (int64, error) {
	var draftID string
	if err := tx.QueryRow(ctx,
		`SELECT id FROM drafts WHERE application_id = $1 AND environment_id = $2`,
		appID, envID).Scan(&draftID); err != nil {
		return 0, err
	}
	return bumpDraftTx(ctx, tx, draftID)
}

func bumpDraftTx(ctx context.Context, tx pgx.Tx, draftID string) (int64, error) {
	var version int64
	if err := tx.QueryRow(ctx,
		`UPDATE drafts SET version = version + 1, updated_at = now() WHERE id = $1 RETURNING version`,
		draftID).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
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
	rows, err := s.pool.Query(ctx, `
		SELECT ng.id, ng.name, COALESCE(array_agg(gm.node_id) FILTER (WHERE gm.node_id IS NOT NULL), '{}')
		FROM node_groups ng
		LEFT JOIN group_members gm ON gm.group_id = ng.id
		GROUP BY ng.id ORDER BY ng.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NodeGroup{}
	for rows.Next() {
		var g NodeGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.MemberIDs); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) CreateNodeGroup(ctx context.Context, id, name string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO node_groups (id, name) VALUES ($1, $2)`, id, name)
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
	if _, err := tx.Exec(ctx,
		`INSERT INTO group_members (group_id, node_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, groupID, nodeID); err != nil {
		if strings.Contains(err.Error(), "violates foreign key constraint") {
			return ErrNotFound
		}
		return err
	}
	// A Managed Node may join several groups, but never two groups that
	// deliver the same Secret Bundle (ADR-0018).
	var ambiguous bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM assignments a
			WHERE a.group_id = $1 AND a.status = 'active'
			  AND EXISTS (
				SELECT 1 FROM assignments a2
				JOIN group_members gm2 ON gm2.group_id = a2.group_id
				WHERE gm2.node_id = $2 AND a2.status = 'active'
				  AND a2.application_id = a.application_id
				  AND a2.environment_id = a.environment_id
				  AND a2.group_id <> a.group_id
			  )
		)`, groupID, nodeID).Scan(&ambiguous); err != nil {
		return err
	}
	if ambiguous {
		return ErrConflict
	}
	return tx.Commit(ctx)
}

func (s *Store) RemoveGroupMember(ctx context.Context, groupID, nodeID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND node_id = $2`, groupID, nodeID)
	return err
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
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.group_id, ng.name, a.application_id, a.environment_id, a.revision_id, a.status,
		       to_char(a.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM assignments a JOIN node_groups ng ON ng.id = a.group_id
		ORDER BY a.created_at DESC, a.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Assignment{}
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.ID, &a.GroupID, &a.GroupName, &a.ApplicationID,
			&a.EnvironmentID, &a.RevisionID, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateAssignment relates a Node Group to a Secret Bundle and resolves the
// Bundle's current Desired Revision. The invariant that one Managed Node
// never receives the same Bundle from two Assignment sources is enforced
// transactionally here and in AddGroupMember.
func (s *Store) CreateAssignment(ctx context.Context, id, groupID, appID, envID string) (*Assignment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var groupName string
	if err := tx.QueryRow(ctx, `SELECT name FROM node_groups WHERE id = $1`, groupID).Scan(&groupName); err != nil {
		return nil, mapNoRows(err)
	}
	var envExists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM environments WHERE id = $1 AND application_id = $2)`,
		envID, appID).Scan(&envExists); err != nil {
		return nil, err
	}
	if !envExists {
		return nil, ErrNotFound
	}
	hasPolicy, err := s.HasActivationPolicy(ctx, envID)
	if err != nil {
		return nil, err
	}
	if !hasPolicy {
		return nil, ErrPolicy
	}
	var revisionID string
	if err := tx.QueryRow(ctx,
		`SELECT id FROM bundle_revisions
		 WHERE application_id = $1 AND environment_id = $2
		 ORDER BY created_at DESC, id DESC LIMIT 1`, appID, envID).Scan(&revisionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBadPayload
		}
		return nil, err
	}
	var ambiguous bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM group_members gm
			JOIN assignments a2 ON a2.group_id = gm.group_id
			  AND a2.application_id = $1 AND a2.environment_id = $2
			  AND a2.status = 'active' AND a2.group_id <> $3
			WHERE gm.node_id IN (SELECT node_id FROM group_members WHERE group_id = $3)
		)`, appID, envID, groupID).Scan(&ambiguous); err != nil {
		return nil, err
	}
	if ambiguous {
		return nil, ErrConflict
	}
	var createdAt string
	if err := tx.QueryRow(ctx, `
		INSERT INTO assignments (id, group_id, application_id, environment_id, revision_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
		id, groupID, appID, envID, revisionID).Scan(&createdAt); err != nil {
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

// AdvanceDesiredRevision points every active Assignment for a Secret Bundle
// at the newly published or rolled-back Revision (ADR-0018).
func (s *Store) AdvanceDesiredRevision(ctx context.Context, appID, envID, revisionID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE assignments SET revision_id = $3
		 WHERE application_id = $1 AND environment_id = $2 AND status = 'active'`,
		appID, envID, revisionID)
	return err
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
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, serial, created_at, last_seen_at, desired_etag, observed_revision, last_result
		FROM nodes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Node{}
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Name, &n.Serial, &n.CreatedAt, &n.LastSeenAt,
			&n.DesiredETag, &n.ObservedRevision, &n.LastResult); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) NodeBySerial(ctx context.Context, serial string) (*Node, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, serial, age_pubkey, created_at, last_seen_at, desired_etag, observed_revision, last_result
		FROM nodes WHERE serial = $1`, serial)
	var n Node
	if err := row.Scan(&n.ID, &n.Name, &n.Serial, &n.AgePubkey, &n.CreatedAt, &n.LastSeenAt,
		&n.DesiredETag, &n.ObservedRevision, &n.LastResult); err != nil {
		return nil, mapNoRows(err)
	}
	return &n, nil
}

func (s *Store) TouchNode(ctx context.Context, nodeID string, observedRevision, lastResult string, at time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE nodes SET last_seen_at = $4, observed_revision = $2, last_result = $3
		WHERE id = $1`, nodeID, observedRevision, lastResult, at)
	return err
}

func (s *Store) SetNodeDesired(ctx context.Context, nodeID, etag string) error {
	_, err := s.pool.Exec(ctx, `UPDATE nodes SET desired_etag = $2 WHERE id = $1`, nodeID, etag)
	return err
}

// --- Enrollment tokens -----------------------------------------------------

type EnrollmentToken struct {
	TokenHash string    `json:"-"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Store) CreateEnrollmentToken(ctx context.Context, tokenHash, name string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO enrollment_tokens (token_hash, name, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, name, expiresAt)
	return err
}

// ConsumeEnrollmentToken atomically validates and consumes a Token, returning
// the Token row on success. Only hashes are stored; the caller presents the
// raw Token once.
func (s *Store) ConsumeEnrollmentToken(ctx context.Context, tokenHash string, now time.Time) (*EnrollmentToken, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var t EnrollmentToken
	if err := tx.QueryRow(ctx,
		`SELECT token_hash, name, created_at, expires_at FROM enrollment_tokens
		 WHERE token_hash = $1 FOR UPDATE`, tokenHash).Scan(&t.TokenHash, &t.Name, &t.CreatedAt, &t.ExpiresAt); err != nil {
		return nil, mapNoRows(err)
	}
	if !t.ExpiresAt.After(now) {
		return nil, ErrConflict
	}
	var used *time.Time
	if err := tx.QueryRow(ctx, `SELECT used_at FROM enrollment_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&used); err != nil {
		return nil, err
	}
	if used != nil {
		return nil, ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE enrollment_tokens SET used_at = $2 WHERE token_hash = $1`,
		tokenHash, now); err != nil {
		return nil, err
	}
	return &t, tx.Commit(ctx)
}

// RegisterNode records an enrolled Managed Node.
func (s *Store) RegisterNode(ctx context.Context, id, name, serial, agePubkey, certPEM string, certExpiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO nodes (id, name, serial, age_pubkey, cert_pem, cert_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, id, name, serial, agePubkey, certPEM, certExpiresAt)
	return err
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

// AssignedRevisions returns the revisions assigned to a node via its groups,
// deduplicated by (application_id, environment_id) with the newest winning
// when a node sits in overlapping groups (the ambiguity rule is a later
// phase; the newest revision wins for now).
func (s *Store) AssignedRevisions(ctx context.Context, nodeID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT a.revision_id
		FROM assignments a
		JOIN group_members gm ON gm.group_id = a.group_id
		WHERE gm.node_id = $1
		  AND a.status = 'active'
		ORDER BY a.revision_id`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) RevisionFiles(ctx context.Context, revisionID string) ([]RevisionFile, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT revision_id, secret_id, path, uid, gid, mode, version_seq
		FROM revision_files WHERE revision_id = $1 ORDER BY path`, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RevisionFile{}
	for rows.Next() {
		var f RevisionFile
		if err := rows.Scan(&f.RevisionID, &f.SecretID, &f.Path, &f.UID, &f.GID, &f.Mode, &f.VersionSeq); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// RevisionVersionMap returns the secret→version selection frozen in a
// Bundle Revision. Core uses it to derive which Secret Version a node
// currently has activated (the node reports only the revision id).
func (s *Store) RevisionVersionMap(ctx context.Context, revisionID string) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT secret_id, version_seq FROM revision_files WHERE revision_id = $1`, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var secretID string
		var seq int64
		if err := rows.Scan(&secretID, &seq); err != nil {
			return nil, err
		}
		out[secretID] = seq
	}
	return out, rows.Err()
}

// SecretVersionSeqs lists the version sequences of a Secret in ascending
// order. Rotatable rotation walks this list cyclically.
func (s *Store) SecretVersionSeqs(ctx context.Context, secretID string) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT seq FROM secret_versions WHERE secret_id = $1 ORDER BY seq`, secretID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var seq int64
		if err := rows.Scan(&seq); err != nil {
			return nil, err
		}
		out = append(out, seq)
	}
	return out, rows.Err()
}

// RevisionAppEnv returns the Application and Environment IDs of a revision.
func (s *Store) RevisionAppEnv(ctx context.Context, revisionID string) (appID, envID string, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT application_id, environment_id FROM bundle_revisions WHERE id = $1`, revisionID).
		Scan(&appID, &envID)
	return appID, envID, mapNoRows(err)
}

// SecretVersionValue decrypts nothing here; it returns the ciphertext blob
// plus the store, which the caller decrypts with the master key.
func (s *Store) SecretVersionBlob(ctx context.Context, secretID string, seq int64) (wrappedKey, nonces, ciphertext []byte, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT wrapped_key, nonce, ciphertext FROM secret_versions
		 WHERE secret_id = $1 AND seq = $2`, secretID, seq).
		Scan(&wrappedKey, &nonces, &ciphertext)
	return wrappedKey, nonces, ciphertext, mapNoRows(err)
}

// SecretAppEnv returns the Application and Environment ids of a Secret.
func (s *Store) SecretAppEnv(ctx context.Context, secretID string) (appID, envID string, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT application_id, environment_id FROM secrets WHERE id = $1`, secretID).
		Scan(&appID, &envID)
	return appID, envID, mapNoRows(err)
}

// MarkRotation records that an Administrator rotated a Secret to the given
// version. The newest row is the pending rotation target for that Secret.
func (s *Store) MarkRotation(ctx context.Context, secretID string, versionSeq int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO secret_rotations (secret_id, version_seq) VALUES ($1, $2)`,
		secretID, versionSeq)
	return err
}

// PendingRotation returns the most recent rotation target for a Secret, or
// 0 when the Secret was never rotated. Nodes are forced onto the target
// until they have converged to it.
func (s *Store) PendingRotation(ctx context.Context, secretID string) (int64, error) {
	var seq int64
	err := s.pool.QueryRow(ctx,
		`SELECT version_seq FROM secret_rotations
		 WHERE secret_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, secretID).
		Scan(&seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return seq, err
}
