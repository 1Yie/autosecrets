package database

import (
	"context"

	"autosecrets.dev/core/internal/database/gen"
)

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
	row, err := gen.New(s.pool).OverviewCounts(ctx)
	if err != nil {
		return nil, err
	}
	return &OverviewCounts{
		Applications: int(row.Applications),
		Environments: int(row.Environments),
		Secrets:      int(row.Secrets),
		Nodes:        int(row.Nodes),
		NodeGroups:   int(row.NodeGroups),
		Assignments:  int(row.Assignments),
		AuditEvents:  int(row.AuditEvents),
	}, nil
}

// OverviewAttention derives current-state risk facts. No Alert records are
// written; the list is recomputed on every request.
func (s *Store) OverviewAttention(ctx context.Context) ([]AttentionFact, error) {
	q := gen.New(s.pool)
	facts := []AttentionFact{}

	counts, err := q.OverviewAttentionCounts(ctx)
	if err != nil {
		return nil, err
	}
	if counts.Failed > 0 {
		facts = append(facts, AttentionFact{Kind: "failed_convergence", Count: int(counts.Failed)})
	}
	if counts.Offline > 0 {
		facts = append(facts, AttentionFact{Kind: "offline_node", Count: int(counts.Offline)})
	}
	if counts.UnassignedNodes > 0 {
		facts = append(facts, AttentionFact{Kind: "unassigned_node", Count: int(counts.UnassignedNodes)})
	}

	cleanup, err := q.OverviewCleanupCounts(ctx)
	if err != nil {
		return nil, err
	}
	// cleanup_unconfirmed is the highest-priority condition: local Secret
	// material may still exist and the control plane stopped waiting.
	if cleanup.CleanupUnconfirmed > 0 {
		facts = append(facts, AttentionFact{Kind: "cleanup_unconfirmed", Count: int(cleanup.CleanupUnconfirmed)})
	}
	if cleanup.CleanupFailed > 0 {
		facts = append(facts, AttentionFact{Kind: "cleanup_failed", Count: int(cleanup.CleanupFailed)})
	}

	envs, err := q.UnclassifiedEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range envs {
		facts = append(facts, AttentionFact{
			Kind: "unclassified_environment", Count: 1, Resource: e.ID,
		})
	}
	return facts, nil
}
