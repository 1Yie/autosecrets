package database

import (
	"context"
	"time"

	"autosecrets.dev/core/internal/database/gen"
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
	return s.q.RecordConvergence(ctx, gen.RecordConvergenceParams{
		NodeID:          row.NodeID,
		AssignmentID:    row.AssignmentID,
		ApplicationID:   row.ApplicationID,
		EnvironmentID:   row.EnvironmentID,
		DesiredRevision: row.DesiredRevision,
		Column6:         row.ObservedRevision,
		Stage:           row.Stage,
		Result:          row.Result,
		Error:           row.Error,
		ReportedAt:      pgTS(row.ReportedAt),
	})
}

// AssignmentForNodeRevision resolves the active Assignment that currently
// delivers the reported revision to a node.
func (s *Store) AssignmentForNodeRevision(ctx context.Context, nodeID, revisionID string) (*Assignment, error) {
	row, err := s.q.AssignmentForNodeRevision(ctx, gen.AssignmentForNodeRevisionParams{
		NodeID:     nodeID,
		RevisionID: revisionID,
	})
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &Assignment{
		ID: row.ID, GroupID: row.GroupID, GroupName: row.Name,
		ApplicationID: row.ApplicationID, EnvironmentID: row.EnvironmentID,
		RevisionID: row.RevisionID, Status: row.Status, CreatedAt: row.CreatedAt,
	}, nil
}

// NodeConvergence returns the per-Assignment rows of one Managed Node,
// failed records first so node detail supports diagnosis (ADR-0015).
func (s *Store) NodeConvergence(ctx context.Context, nodeID string) ([]ConvergenceRow, error) {
	rows, err := s.q.NodeConvergence(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return convergenceRows(rows), nil
}

// AllNodeConvergence returns every convergence row grouped by node for the
// fleet projection in one query.
func (s *Store) AllNodeConvergence(ctx context.Context) (map[string][]ConvergenceRow, error) {
	rows, err := s.q.AllNodeConvergence(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string][]ConvergenceRow{}
	for _, r := range rows {
		out[r.NodeID] = append(out[r.NodeID], ConvergenceRow{
			NodeID: r.NodeID, AssignmentID: r.AssignmentID, ApplicationID: r.ApplicationID,
			EnvironmentID: r.EnvironmentID, DesiredRevision: r.DesiredRevision,
			ObservedRevision: r.ObservedRevision, Stage: r.Stage, Result: r.Result,
			Error: r.Error, ReportedAt: tsPtr(r.ReportedAt),
		})
	}
	return out, nil
}

// NodeAssignmentCounts reports how many active Assignments each node
// receives, used to distinguish healthy-unassigned from converging.
func (s *Store) NodeAssignmentCounts(ctx context.Context) (map[string]int, error) {
	rows, err := s.q.NodeAssignmentCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, r := range rows {
		out[r.NodeID] = int(r.Count)
	}
	return out, nil
}

func convergenceRows(rows []gen.NodeConvergenceRow) []ConvergenceRow {
	out := make([]ConvergenceRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ConvergenceRow{
			NodeID: r.NodeID, AssignmentID: r.AssignmentID, ApplicationID: r.ApplicationID,
			EnvironmentID: r.EnvironmentID, DesiredRevision: r.DesiredRevision,
			ObservedRevision: r.ObservedRevision, Stage: r.Stage, Result: r.Result,
			Error: r.Error, ReportedAt: tsPtr(r.ReportedAt),
		})
	}
	return out
}
