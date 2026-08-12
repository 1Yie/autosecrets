package store

import (
	"context"
	"time"
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
	if _, err := tx.Exec(ctx, `
		INSERT INTO activation_policies (environment_id, action)
		VALUES ($1, $2)
		ON CONFLICT (environment_id) DO UPDATE SET action = EXCLUDED.action, updated_at = now()`,
		policy.EnvironmentID, policy.Action); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM activation_policy_units WHERE environment_id = $1`, policy.EnvironmentID); err != nil {
		return err
	}
	for i, unit := range policy.Units {
		if _, err := tx.Exec(ctx,
			`INSERT INTO activation_policy_units (environment_id, position, unit_name) VALUES ($1, $2, $3)`,
			policy.EnvironmentID, i+1, unit); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) GetActivationPolicy(ctx context.Context, envID string) (*ActivationPolicy, error) {
	var policy ActivationPolicy
	if err := s.pool.QueryRow(ctx,
		`SELECT environment_id, action FROM activation_policies WHERE environment_id = $1`, envID).
		Scan(&policy.EnvironmentID, &policy.Action); err != nil {
		return nil, mapNoRows(err)
	}
	rows, err := s.pool.Query(ctx,
		`SELECT unit_name FROM activation_policy_units WHERE environment_id = $1 ORDER BY position`, envID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var unit string
		if err := rows.Scan(&unit); err != nil {
			return nil, err
		}
		policy.Units = append(policy.Units, unit)
	}
	return &policy, rows.Err()
}

func (s *Store) HasActivationPolicy(ctx context.Context, envID string) (bool, error) {
	var has bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM activation_policies WHERE environment_id = $1)`, envID).Scan(&has)
	return has, err
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
	tag, err := tx.Exec(ctx,
		`UPDATE assignments SET status = 'removing' WHERE id = $1 AND status = 'active'`, assignmentID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, ErrConflict
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO unassignment_tasks (assignment_id, node_id)
		SELECT $1, gm.node_id FROM group_members gm WHERE gm.group_id = (
			SELECT group_id FROM assignments WHERE id = $1)`,
		assignmentID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT assignment_id, node_id, status, error, updated_at
		FROM unassignment_tasks WHERE assignment_id = $1 ORDER BY node_id`, assignmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []UnassignmentTask{}
	for rows.Next() {
		var task UnassignmentTask
		if err := rows.Scan(&task.AssignmentID, &task.NodeID, &task.Status, &task.Error, &task.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, tx.Commit(ctx)
}

func (s *Store) ListUnassignmentTasks(ctx context.Context, assignmentID string) ([]UnassignmentTask, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT assignment_id, node_id, status, error, updated_at
		FROM unassignment_tasks WHERE assignment_id = $1 ORDER BY node_id`, assignmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []UnassignmentTask{}
	for rows.Next() {
		var task UnassignmentTask
		if err := rows.Scan(&task.AssignmentID, &task.NodeID, &task.Status, &task.Error, &task.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// ReportCleanupTask records the Agent's cleanup acknowledgement. Pending and
// failed tasks may transition, and a reconnecting node may clear a
// cleanup_unconfirmed task by actually completing the local cleanup.
func (s *Store) ReportCleanupTask(ctx context.Context, assignmentID, nodeID, result, errorMsg string) error {
	if result != "cleaned" && result != "failed" {
		return ErrBadPayload
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE unassignment_tasks
		SET status = $3, error = $4, updated_at = now()
		WHERE assignment_id = $1 AND node_id = $2
		  AND status IN ('pending', 'failed', 'cleanup_unconfirmed')`,
		assignmentID, nodeID, result, errorMsg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

// AbandonCleanupConfirmation ends control-plane waiting for unreachable or
// failed nodes. The result is never "success": per-node uncertainty is
// retained as cleanup_unconfirmed, which stays a highest-priority attention
// item until the operator resolves the node locally.
func (s *Store) AbandonCleanupConfirmation(ctx context.Context, assignmentID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE unassignment_tasks SET status = 'cleanup_unconfirmed', updated_at = now()
		WHERE assignment_id = $1 AND status IN ('pending', 'failed', 'offline')`, assignmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) AssignmentByID(ctx context.Context, assignmentID string) (*Assignment, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT a.id, a.group_id, ng.name, a.application_id, a.environment_id, a.revision_id, a.status,
		       to_char(a.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM assignments a JOIN node_groups ng ON ng.id = a.group_id
		WHERE a.id = $1`, assignmentID)
	var a Assignment
	if err := row.Scan(&a.ID, &a.GroupID, &a.GroupName, &a.ApplicationID,
		&a.EnvironmentID, &a.RevisionID, &a.Status, &a.CreatedAt); err != nil {
		return nil, mapNoRows(err)
	}
	return &a, nil
}

// PendingCleanupTasks returns the cleanup instructions a node must process
// before any new Desired State, with the units to stop in reverse order.
type CleanupInstruction struct {
	AssignmentID  string
	ApplicationID string
	EnvironmentID string
	Units         []string
}

func (s *Store) PendingCleanupInstructions(ctx context.Context, nodeID string) ([]CleanupInstruction, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.assignment_id, a.application_id, a.environment_id
		FROM unassignment_tasks t
		JOIN assignments a ON a.id = t.assignment_id
		WHERE t.node_id = $1 AND t.status IN ('pending', 'failed', 'cleanup_unconfirmed')
		ORDER BY t.assignment_id`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CleanupInstruction{}
	for rows.Next() {
		var instruction CleanupInstruction
		if err := rows.Scan(&instruction.AssignmentID, &instruction.ApplicationID, &instruction.EnvironmentID); err != nil {
			return nil, err
		}
		policy, err := s.GetActivationPolicy(ctx, instruction.EnvironmentID)
		if err == nil {
			instruction.Units = policy.Units
		}
		out = append(out, instruction)
	}
	return out, rows.Err()
}
