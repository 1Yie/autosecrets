package store

import "context"

// SearchResult is one global-search hit. Search covers only Applications,
// Environments, Managed Nodes, and Node Groups (never Secret names or Audit
// Events).
type SearchResult struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Store) Search(ctx context.Context, query string) ([]SearchResult, error) {
	query = "%" + query + "%"
	results := []SearchResult{}
	collect := func(sql string, resultType string) error {
		rows, err := s.pool.Query(ctx, sql, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var result SearchResult
			result.Type = resultType
			if err := rows.Scan(&result.ID, &result.Name); err != nil {
				return err
			}
			results = append(results, result)
		}
		return rows.Err()
	}
	if err := collect(
		`SELECT id, name FROM applications WHERE name ILIKE $1 ORDER BY name LIMIT 20`,
		"application"); err != nil {
		return nil, err
	}
	if err := collect(`
		SELECT e.id, a.name || '/' || e.name FROM environments e
		JOIN applications a ON a.id = e.application_id
		WHERE e.name ILIKE $1 ORDER BY e.name LIMIT 20`, "environment"); err != nil {
		return nil, err
	}
	if err := collect(`
		SELECT id, name FROM nodes WHERE name ILIKE $1 OR serial ILIKE $1 ORDER BY name LIMIT 20`,
		"node"); err != nil {
		return nil, err
	}
	if err := collect(
		`SELECT id, name FROM node_groups WHERE name ILIKE $1 ORDER BY name LIMIT 20`,
		"node_group"); err != nil {
		return nil, err
	}
	return results, nil
}
