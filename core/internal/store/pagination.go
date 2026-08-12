package store

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cursor is an opaque keyset position: the last visible row's created time
// and id. Encode/decode lives here so no caller reimplements the wire
// format.
type Cursor struct {
	At time.Time
	ID string
}

func EncodeCursor(at time.Time, id string) string {
	raw := fmt.Sprintf("%d|%s", at.UTC().UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(value string) (Cursor, error) {
	if value == "" {
		return Cursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Cursor{}, err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return Cursor{}, errors.New("store: malformed cursor")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Cursor{}, err
	}
	return Cursor{At: time.Unix(0, nanos).UTC(), ID: parts[1]}, nil
}

// ListApplicationsPage returns cursor-paginated Applications ordered by
// created_at DESC, id DESC.
func (s *Store) ListApplicationsPage(ctx context.Context, cursor Cursor, limit int) ([]Application, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := `SELECT id, name, created_at FROM applications`
	args := []any{}
	if cursor.ID != "" {
		args = append(args, cursor.At, cursor.ID)
		query += ` WHERE (created_at < $1 OR (created_at = $1 AND id < $2))`
	}
	args = append(args, limit+1)
	query += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d`, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []Application{}
	for rows.Next() {
		var application Application
		if err := rows.Scan(&application.ID, &application.Name, &application.CreatedAt); err != nil {
			return nil, "", err
		}
		out = append(out, application)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	return trimPage(out, limit, func(application Application) (time.Time, string) {
		return application.CreatedAt, application.ID
	})
}

// ListNodeGroupsPage returns cursor-paginated Node Groups ordered by
// created_at DESC, id DESC.
func (s *Store) ListNodeGroupsPage(ctx context.Context, cursor Cursor, limit int) ([]NodeGroup, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := `
		SELECT ng.id, ng.name, ng.created_at,
		       COALESCE(array_agg(gm.node_id) FILTER (WHERE gm.node_id IS NOT NULL), '{}')
		FROM node_groups ng
		LEFT JOIN group_members gm ON gm.group_id = ng.id
		GROUP BY ng.id`
	args := []any{}
	if cursor.ID != "" {
		args = append(args, cursor.At, cursor.ID)
		query += ` WHERE (ng.created_at < $1 OR (ng.created_at = $1 AND ng.id < $2))`
	}
	args = append(args, limit+1)
	query += fmt.Sprintf(` ORDER BY ng.created_at DESC, ng.id DESC LIMIT $%d`, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []NodeGroup{}
	for rows.Next() {
		var group NodeGroup
		if err := rows.Scan(&group.ID, &group.Name, &group.CreatedAt, &group.MemberIDs); err != nil {
			return nil, "", err
		}
		out = append(out, group)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	return trimPage(out, limit, func(group NodeGroup) (time.Time, string) {
		return group.CreatedAt, group.ID
	})
}

// ListAssignmentsPage returns cursor-paginated Assignments ordered by
// created_at DESC, id DESC.
func (s *Store) ListAssignmentsPage(ctx context.Context, cursor Cursor, limit int) ([]Assignment, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := `
		SELECT a.id, a.group_id, ng.name, a.application_id, a.environment_id, a.revision_id, a.status,
		       a.created_at
		FROM assignments a JOIN node_groups ng ON ng.id = a.group_id
		`
	args := []any{}
	if cursor.ID != "" {
		args = append(args, cursor.At, cursor.ID)
		query += ` WHERE (a.created_at < $1 OR (a.created_at = $1 AND a.id < $2))`
	}
	args = append(args, limit+1)
	query += fmt.Sprintf(` ORDER BY a.created_at DESC, a.id DESC LIMIT $%d`, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []Assignment{}
	for rows.Next() {
		var assignment Assignment
		var createdAt time.Time
		if err := rows.Scan(&assignment.ID, &assignment.GroupID, &assignment.GroupName,
			&assignment.ApplicationID, &assignment.EnvironmentID, &assignment.RevisionID,
			&assignment.Status, &createdAt); err != nil {
			return nil, "", err
		}
		assignment.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		out = append(out, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	return trimPage(out, limit, func(assignment Assignment) (time.Time, string) {
		at, _ := time.Parse(time.RFC3339, assignment.CreatedAt)
		return at, assignment.ID
	})
}

func trimPage[T any](items []T, limit int, key func(T) (time.Time, string)) ([]T, string, error) {
	if len(items) <= limit {
		return items, "", nil
	}
	at, id := key(items[limit])
	return items[:limit], EncodeCursor(at, id), nil
}
