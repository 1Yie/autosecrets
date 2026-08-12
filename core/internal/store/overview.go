package store

import "context"

// OverviewCounts is the compact asset and health summary behind the
// /overview projection.
type OverviewCounts struct {
	Applications int `json:"applications"`
	Environments int `json:"environments"`
	Secrets      int `json:"secrets"`
	Nodes        int `json:"nodes"`
	NodeGroups   int `json:"node_groups"`
	Assignments  int `json:"assignments"`
	AuditEvents  int `json:"audit_events"`
}

type AttentionFact struct {
	Kind     string `json:"kind"`
	Count    int    `json:"count"`
	Resource string `json:"resource,omitempty"`
}

func (s *Store) OverviewCounts(ctx context.Context) (*OverviewCounts, error) {
	var counts OverviewCounts
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM applications),
			(SELECT count(*) FROM environments),
			(SELECT count(*) FROM secrets WHERE retired_at IS NULL),
			(SELECT count(*) FROM nodes),
			(SELECT count(*) FROM node_groups),
			(SELECT count(*) FROM assignments WHERE status = 'active'),
			(SELECT count(*) FROM audit_events)`).
		Scan(&counts.Applications, &counts.Environments, &counts.Secrets,
			&counts.Nodes, &counts.NodeGroups, &counts.Assignments, &counts.AuditEvents)
	return &counts, err
}

// OverviewAttention derives current-state risk facts. No Alert records are
// written; the list is recomputed on every request.
func (s *Store) OverviewAttention(ctx context.Context) ([]AttentionFact, error) {
	facts := []AttentionFact{}

	var failed, offline, unassignedNodes int
	if err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(DISTINCT node_id) FROM node_convergence WHERE result = 'failed'),
			(SELECT count(*) FROM nodes WHERE last_seen_at IS NULL OR last_seen_at < now() - interval '75 seconds'),
			(SELECT count(*) FROM nodes n WHERE NOT EXISTS (
				SELECT 1 FROM group_members gm JOIN assignments a ON a.group_id = gm.group_id
				WHERE gm.node_id = n.id AND a.status = 'active'))`).
		Scan(&failed, &offline, &unassignedNodes); err != nil {
		return nil, err
	}
	if failed > 0 {
		facts = append(facts, AttentionFact{Kind: "failed_convergence", Count: failed})
	}
	if offline > 0 {
		facts = append(facts, AttentionFact{Kind: "offline_node", Count: offline})
	}
	if unassignedNodes > 0 {
		facts = append(facts, AttentionFact{Kind: "unassigned_node", Count: unassignedNodes})
	}

	var cleanupFailed, cleanupUnconfirmed int
	if err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM unassignment_tasks WHERE status = 'failed'),
			(SELECT count(*) FROM unassignment_tasks WHERE status = 'cleanup_unconfirmed')`).
		Scan(&cleanupFailed, &cleanupUnconfirmed); err != nil {
		return nil, err
	}
	if cleanupFailed > 0 {
		facts = append(facts, AttentionFact{Kind: "cleanup_failed", Count: cleanupFailed})
	}
	if cleanupUnconfirmed > 0 {
		facts = append(facts, AttentionFact{Kind: "cleanup_unconfirmed", Count: cleanupUnconfirmed})
	}

	rows, err := s.pool.Query(ctx, `
		SELECT e.id, a.name, e.name
		FROM environments e JOIN applications a ON a.id = e.application_id
		WHERE e.protection_level = 'unclassified'
		ORDER BY a.name, e.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	unclassified := 0
	for rows.Next() {
		var id, appName, envName string
		if err := rows.Scan(&id, &appName, &envName); err != nil {
			return nil, err
		}
		unclassified++
		facts = append(facts, AttentionFact{
			Kind: "unclassified_environment", Count: 1, Resource: id,
		})
	}
	return facts, rows.Err()
}
