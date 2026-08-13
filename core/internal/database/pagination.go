package database

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"autosecrets.dev/core/internal/database/gen"
)

// Cursor is an opaque keyset position: the last visible row's created time
// and id. Encode/decode lives here so no caller reimplements the wire format.
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
	var rows []gen.Application
	var err error
	if cursor.ID == "" {
		rows, err = s.q.ListApplicationsFirstPage(ctx, int32(limit+1))
	} else {
		rows, err = s.q.ListApplicationsNextPage(ctx, gen.ListApplicationsNextPageParams{
			CreatedAt: cursor.At, ID: cursor.ID, Limit: int32(limit + 1),
		})
	}
	if err != nil {
		return nil, "", err
	}
	out := make([]Application, len(rows))
	for i, r := range rows {
		out[i] = Application(r)
	}
	return trimPage(out, limit, func(a Application) (time.Time, string) {
		return a.CreatedAt, a.ID
	})
}

// ListNodeGroupsPage returns cursor-paginated Node Groups ordered by
// created_at DESC, id DESC.
func (s *Store) ListNodeGroupsPage(ctx context.Context, cursor Cursor, limit int) ([]NodeGroup, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	out := []NodeGroup{}
	if cursor.ID == "" {
		rows, err := s.q.ListNodeGroupsFirstPage(ctx, int32(limit+1))
		if err != nil {
			return nil, "", err
		}
		for _, r := range rows {
			out = append(out, NodeGroup{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, MemberIDs: r.MemberIds})
		}
	} else {
		rows, err := s.q.ListNodeGroupsNextPage(ctx, gen.ListNodeGroupsNextPageParams{
			CreatedAt: cursor.At, ID: cursor.ID, Limit: int32(limit + 1),
		})
		if err != nil {
			return nil, "", err
		}
		for _, r := range rows {
			out = append(out, NodeGroup{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, MemberIDs: r.MemberIds})
		}
	}
	return trimPage(out, limit, func(g NodeGroup) (time.Time, string) {
		return g.CreatedAt, g.ID
	})
}

// ListAssignmentsPage returns cursor-paginated Assignments ordered by
// created_at DESC, id DESC.
func (s *Store) ListAssignmentsPage(ctx context.Context, cursor Cursor, limit int) ([]Assignment, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	out := []Assignment{}
	if cursor.ID == "" {
		rows, err := s.q.ListAssignmentsFirstPage(ctx, int32(limit+1))
		if err != nil {
			return nil, "", err
		}
		for _, r := range rows {
			out = append(out, assignmentFromRow(r.ID, r.GroupID, r.Name, r.ApplicationID, r.EnvironmentID, r.RevisionID, r.Status, r.CreatedAt))
		}
	} else {
		rows, err := s.q.ListAssignmentsNextPage(ctx, gen.ListAssignmentsNextPageParams{
			CreatedAt: cursor.At, ID: cursor.ID, Limit: int32(limit + 1),
		})
		if err != nil {
			return nil, "", err
		}
		for _, r := range rows {
			out = append(out, assignmentFromRow(r.ID, r.GroupID, r.Name, r.ApplicationID, r.EnvironmentID, r.RevisionID, r.Status, r.CreatedAt))
		}
	}
	return trimPage(out, limit, func(a Assignment) (time.Time, string) {
		at, _ := time.Parse(time.RFC3339, a.CreatedAt)
		return at, a.ID
	})
}

func assignmentFromRow(id, groupID, groupName, appID, envID, revisionID, status string, createdAt time.Time) Assignment {
	return Assignment{
		ID: id, GroupID: groupID, GroupName: groupName,
		ApplicationID: appID, EnvironmentID: envID, RevisionID: revisionID,
		Status: status, CreatedAt: createdAt.UTC().Format(time.RFC3339),
	}
}

func trimPage[T any](items []T, limit int, key func(T) (time.Time, string)) ([]T, string, error) {
	if len(items) <= limit {
		return items, "", nil
	}
	at, id := key(items[limit])
	return items[:limit], EncodeCursor(at, id), nil
}
