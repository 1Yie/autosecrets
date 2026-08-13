package database

import (
	"context"

	"autosecrets.dev/core/internal/database/gen"
)

// SearchResult is one global-search hit. Search covers only Applications,
// Environments, Managed Nodes, and Node Groups (never Secret names or Audit
// Events).
type SearchResult struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Store) Search(ctx context.Context, query string) ([]SearchResult, error) {
	q := gen.New(s.pool)
	pattern := "%" + query + "%"
	results := []SearchResult{}

	apps, err := q.SearchApplications(ctx, pattern)
	if err != nil {
		return nil, err
	}
	for _, r := range apps {
		results = append(results, SearchResult{Type: "application", ID: r.ID, Name: r.Name})
	}

	envs, err := q.SearchEnvironments(ctx, pattern)
	if err != nil {
		return nil, err
	}
	for _, r := range envs {
		results = append(results, SearchResult{Type: "environment", ID: r.ID, Name: r.Name})
	}

	nodes, err := q.SearchNodes(ctx, pattern)
	if err != nil {
		return nil, err
	}
	for _, r := range nodes {
		results = append(results, SearchResult{Type: "node", ID: r.ID, Name: r.Name})
	}

	groups, err := q.SearchNodeGroups(ctx, pattern)
	if err != nil {
		return nil, err
	}
	for _, r := range groups {
		results = append(results, SearchResult{Type: "node_group", ID: r.ID, Name: r.Name})
	}
	return results, nil
}
