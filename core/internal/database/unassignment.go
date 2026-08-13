package database

import (
	"context"
	"time"

	"autosecrets.dev/core/internal/database/gen"
)

// ActivationPolicy declares the ordered systemd units and the bounded
// post-Activation action for one Environment (ADR-0022).
type ActivationPolicy struct {
	EnvironmentID string
	Action        string
	Units         []string
}

func (s *Store) SaveActivationPolicy(ctx context.Context, policy ActivationPolicy) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	if err := q.UpsertActivationPolicy(ctx, gen.UpsertActivationPolicyParams{
		EnvironmentID: policy.EnvironmentID, Action: policy.Action,
	}); err != nil {
		return err
	}
	if err := q.DeleteActivationPolicyUnits(ctx, policy.EnvironmentID); err != nil {
		return err
	}
	for i, unit := range policy.Units {
		if err := q.InsertActivationPolicyUnit(ctx, gen.InsertActivationPolicyUnitParams{
			EnvironmentID: policy.EnvironmentID, Position: int16(i + 1), UnitName: unit,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) GetActivationPolicy(ctx context.Context, envID string) (*ActivationPolicy, error) {
	row, err := s.q.GetActivationPolicy(ctx, envID)
	if err != nil {
		return nil, mapNoRows(err)
	}
	policy := &ActivationPolicy{EnvironmentID: row.EnvironmentID, Action: row.Action}
	units, err := s.q.ListActivationPolicyUnits(ctx, envID)
	if err != nil {
		return nil, err
	}
	policy.Units = append(policy.Units, units...)
	return policy, nil
}

// UnassignmentTask is the per-node cleanup state of one removed Assignment.
type UnassignmentTask struct {
	AssignmentID string     `json:"assignment_id"`
	NodeID       string     `json:"node_id"`
	Status       string     `json:"status"`
	Error        string     `json:"error,omitempty"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

// Unassign removes Desired State delivery and creates persistent per-node
// cleanup tasks. The Assignment row stays as a removing tombstone.
func (s *Store) Unassign(ctx context.Context, assignmentID string) ([]UnassignmentTask, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	n, err := q.MarkAssignmentRemoving(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrConflict
	}
	if err := q.InsertUnassignmentTasks(ctx, assignmentID); err != nil {
		return nil, err
	}
	rows, err := q.ListUnassignmentTasks(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	tasks := make([]UnassignmentTask, len(rows))
	for i, r := range rows {
		tasks[i] = UnassignmentTask{
			AssignmentID: r.AssignmentID, NodeID: r.NodeID,
			Status: r.Status, Error: r.Error, UpdatedAt: &r.UpdatedAt,
		}
	}
	return tasks, tx.Commit(ctx)
}

func (s *Store) ListUnassignmentTasks(ctx context.Context, assignmentID string) ([]UnassignmentTask, error) {
	rows, err := s.q.ListUnassignmentTasks(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	tasks := make([]UnassignmentTask, len(rows))
	for i, r := range rows {
		tasks[i] = UnassignmentTask{
			AssignmentID: r.AssignmentID, NodeID: r.NodeID,
			Status: r.Status, Error: r.Error, UpdatedAt: &r.UpdatedAt,
		}
	}
	return tasks, nil
}

// ReportCleanupTask records the Agent's cleanup acknowledgement. Pending and
// failed tasks may transition, and a reconnecting node may clear a
// cleanup_unconfirmed task by actually completing the local cleanup.
func (s *Store) ReportCleanupTask(ctx context.Context, assignmentID, nodeID, result, errorMsg string) error {
	if result != "cleaned" && result != "failed" {
		return ErrBadPayload
	}
	n, err := s.q.ReportCleanupTask(ctx, gen.ReportCleanupTaskParams{
		AssignmentID: assignmentID, NodeID: nodeID, Status: result, Error: errorMsg,
	})
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNotFound
	}
	return nil
}

// AbandonCleanupConfirmation ends control-plane waiting for unreachable or
// failed nodes. The result is never "success": per-node uncertainty is
// retained as cleanup_unconfirmed, which stays a highest-priority attention
// item until the operator resolves the node locally.
func (s *Store) AbandonCleanupConfirmation(ctx context.Context, assignmentID string) error {
	n, err := s.q.AbandonCleanupConfirmation(ctx, assignmentID)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) AssignmentByID(ctx context.Context, assignmentID string) (*Assignment, error) {
	row, err := s.q.AssignmentByID(ctx, assignmentID)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &Assignment{
		ID: row.ID, GroupID: row.GroupID, GroupName: row.Name,
		ApplicationID: row.ApplicationID, EnvironmentID: row.EnvironmentID,
		RevisionID: row.RevisionID, Status: row.Status, CreatedAt: row.CreatedAt,
	}, nil
}

// CleanupInstruction carries the cleanup instructions a node must process
// before any new Desired State, with the units to stop in reverse order.
type CleanupInstruction struct {
	AssignmentID  string
	ApplicationID string
	EnvironmentID string
	Units         []string
}

func (s *Store) PendingCleanupInstructions(ctx context.Context, nodeID string) ([]CleanupInstruction, error) {
	rows, err := s.q.PendingCleanupInstructions(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	out := []CleanupInstruction{}
	for _, r := range rows {
		instruction := CleanupInstruction{
			AssignmentID: r.AssignmentID, ApplicationID: r.ApplicationID, EnvironmentID: r.EnvironmentID,
		}
		policy, err := s.GetActivationPolicy(ctx, r.EnvironmentID)
		if err == nil {
			instruction.Units = policy.Units
		}
		out = append(out, instruction)
	}
	return out, nil
}
