package store

import (
	"context"
	"time"
)

// ConvergenceRow is the persisted per-Assignment Convergence state of one
// Managed Node (ADR-0015).
type ConvergenceRow struct {
	NodeID           string
	AssignmentID     string
	ApplicationID    string
	EnvironmentID    string
	DesiredRevision  string
	ObservedRevision string
	Stage            string
	Result           string
	Error            string
	ReportedAt       *time.Time
}

// RecordConvergence upserts one node + assignment pair. The observed
// revision only moves on success; a failed Activation keeps the last known
// observed value so partial success stays visible.
func (s *Store) RecordConvergence(ctx context.Context, row ConvergenceRow) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO node_convergence
			(node_id, assignment_id, application_id, environment_id,
			 desired_revision, observed_revision, stage, result, error, reported_at)
		VALUES ($1, $2, $3, $4, $5,
			CASE WHEN $8 = 'ok' THEN $6 ELSE '' END, $7, $8, $9, $10)
		ON CONFLICT (node_id, assignment_id) DO UPDATE SET
			application_id = EXCLUDED.application_id,
			environment_id = EXCLUDED.environment_id,
			desired_revision = EXCLUDED.desired_revision,
			observed_revision = CASE
				WHEN EXCLUDED.result = 'ok' THEN EXCLUDED.observed_revision
				ELSE node_convergence.observed_revision END,
			stage = EXCLUDED.stage,
			result = EXCLUDED.result,
			error = EXCLUDED.error,
			reported_at = EXCLUDED.reported_at,
			updated_at = now()`,
		row.NodeID, row.AssignmentID, row.ApplicationID, row.EnvironmentID,
		row.DesiredRevision, row.ObservedRevision, row.Stage, row.Result, row.Error, row.ReportedAt)
	return err
}

// AssignmentForNodeRevision resolves the active Assignment that currently
// delivers the reported revision to a node.
func (s *Store) AssignmentForNodeRevision(ctx context.Context, nodeID, revisionID string) (*Assignment, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT a.id, a.group_id, ng.name, a.application_id, a.environment_id, a.revision_id, a.status,
		       to_char(a.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM assignments a
		JOIN group_members gm ON gm.group_id = a.group_id
		JOIN node_groups ng ON ng.id = a.group_id
		WHERE gm.node_id = $1 AND a.revision_id = $2 AND a.status = 'active'
		LIMIT 1`, nodeID, revisionID)
	var a Assignment
	if err := row.Scan(&a.ID, &a.GroupID, &a.GroupName, &a.ApplicationID,
		&a.EnvironmentID, &a.RevisionID, &a.Status, &a.CreatedAt); err != nil {
		return nil, mapNoRows(err)
	}
	return &a, nil
}

// NodeConvergence returns the per-Assignment rows of one Managed Node,
// failed records first so node detail supports diagnosis (ADR-0015).
func (s *Store) NodeConvergence(ctx context.Context, nodeID string) ([]ConvergenceRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT node_id, assignment_id, application_id, environment_id,
		       desired_revision, observed_revision, stage, result, error, reported_at
		FROM node_convergence WHERE node_id = $1
		ORDER BY (result = 'failed') DESC, updated_at DESC`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ConvergenceRow{}
	for rows.Next() {
		var row ConvergenceRow
		if err := rows.Scan(&row.NodeID, &row.AssignmentID, &row.ApplicationID, &row.EnvironmentID,
			&row.DesiredRevision, &row.ObservedRevision, &row.Stage, &row.Result, &row.Error,
			&row.ReportedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// AllNodeConvergence returns every convergence row grouped by node for the
// fleet projection in one query.
func (s *Store) AllNodeConvergence(ctx context.Context) (map[string][]ConvergenceRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT node_id, assignment_id, application_id, environment_id,
		       desired_revision, observed_revision, stage, result, error, reported_at
		FROM node_convergence ORDER BY node_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]ConvergenceRow{}
	for rows.Next() {
		var row ConvergenceRow
		if err := rows.Scan(&row.NodeID, &row.AssignmentID, &row.ApplicationID, &row.EnvironmentID,
			&row.DesiredRevision, &row.ObservedRevision, &row.Stage, &row.Result, &row.Error,
			&row.ReportedAt); err != nil {
			return nil, err
		}
		out[row.NodeID] = append(out[row.NodeID], row)
	}
	return out, rows.Err()
}

// NodeAssignmentCounts reports how many active Assignments each node
// receives, used to distinguish healthy-unassigned from converging.
func (s *Store) NodeAssignmentCounts(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT gm.node_id, count(*)
		FROM group_members gm
		JOIN assignments a ON a.group_id = gm.group_id
		WHERE a.status = 'active'
		GROUP BY gm.node_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var nodeID string
		var count int
		if err := rows.Scan(&nodeID, &count); err != nil {
			return nil, err
		}
		out[nodeID] = count
	}
	return out, rows.Err()
}
