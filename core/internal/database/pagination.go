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

func clampLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 25
	}
	return limit
}

func useOffset(cursor Cursor, page int) bool {
	return page > 1 && cursor.ID == ""
}

func pageOffset(page, limit int) int {
	if page <= 1 {
		return 0
	}
	return (page - 1) * limit
}

// ListApplicationsPage returns paginated Applications ordered by
// created_at DESC, id DESC. page is 1-based; values above 1 use offset.
func (s *Store) ListApplicationsPage(ctx context.Context, cursor Cursor, limit, page int) ([]Application, string, int64, error) {
	limit = clampLimit(limit)
	total, err := s.q.CountApplications(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	var rows []gen.Application
	if useOffset(cursor, page) {
		rows, err = s.q.ListApplicationsOffsetPage(ctx, gen.ListApplicationsOffsetPageParams{
			Limit: int32(limit + 1), Offset: int32(pageOffset(page, limit)),
		})
	} else if cursor.ID == "" {
		rows, err = s.q.ListApplicationsFirstPage(ctx, int32(limit+1))
	} else {
		rows, err = s.q.ListApplicationsNextPage(ctx, gen.ListApplicationsNextPageParams{
			CreatedAt: cursor.At, ID: cursor.ID, Limit: int32(limit + 1),
		})
	}
	if err != nil {
		return nil, "", 0, err
	}
	out := make([]Application, len(rows))
	for i, r := range rows {
		out[i] = Application(r)
	}
	items, next, err := trimPage(out, limit, func(a Application) (time.Time, string) {
		return a.CreatedAt, a.ID
	})
	return items, next, total, err
}

// ListNodeGroupsPage returns paginated Node Groups ordered by
// created_at DESC, id DESC.
func (s *Store) ListNodeGroupsPage(ctx context.Context, cursor Cursor, limit, page int) ([]NodeGroup, string, int64, error) {
	limit = clampLimit(limit)
	total, err := s.q.CountNodeGroups(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	out := []NodeGroup{}
	if useOffset(cursor, page) {
		rows, err := s.q.ListNodeGroupsOffsetPage(ctx, gen.ListNodeGroupsOffsetPageParams{
			Limit: int32(limit + 1), Offset: int32(pageOffset(page, limit)),
		})
		if err != nil {
			return nil, "", 0, err
		}
		for _, r := range rows {
			out = append(out, NodeGroup{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, MemberIDs: r.MemberIds})
		}
	} else if cursor.ID == "" {
		rows, err := s.q.ListNodeGroupsFirstPage(ctx, int32(limit+1))
		if err != nil {
			return nil, "", 0, err
		}
		for _, r := range rows {
			out = append(out, NodeGroup{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, MemberIDs: r.MemberIds})
		}
	} else {
		rows, err := s.q.ListNodeGroupsNextPage(ctx, gen.ListNodeGroupsNextPageParams{
			CreatedAt: cursor.At, ID: cursor.ID, Limit: int32(limit + 1),
		})
		if err != nil {
			return nil, "", 0, err
		}
		for _, r := range rows {
			out = append(out, NodeGroup{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, MemberIDs: r.MemberIds})
		}
	}
	items, next, err := trimPage(out, limit, func(g NodeGroup) (time.Time, string) {
		return g.CreatedAt, g.ID
	})
	return items, next, total, err
}

// ListAssignmentsPage returns paginated Assignments ordered by
// created_at DESC, id DESC.
func (s *Store) ListAssignmentsPage(ctx context.Context, cursor Cursor, limit, page int) ([]Assignment, string, int64, error) {
	limit = clampLimit(limit)
	total, err := s.q.CountAssignments(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	out := []Assignment{}
	if useOffset(cursor, page) {
		rows, err := s.q.ListAssignmentsOffsetPage(ctx, gen.ListAssignmentsOffsetPageParams{
			Limit: int32(limit + 1), Offset: int32(pageOffset(page, limit)),
		})
		if err != nil {
			return nil, "", 0, err
		}
		for _, r := range rows {
			out = append(out, assignmentFromRow(r.ID, r.GroupID, r.Name, r.ApplicationID, r.EnvironmentID, r.RevisionID, r.Status, r.CreatedAt))
		}
	} else if cursor.ID == "" {
		rows, err := s.q.ListAssignmentsFirstPage(ctx, int32(limit+1))
		if err != nil {
			return nil, "", 0, err
		}
		for _, r := range rows {
			out = append(out, assignmentFromRow(r.ID, r.GroupID, r.Name, r.ApplicationID, r.EnvironmentID, r.RevisionID, r.Status, r.CreatedAt))
		}
	} else {
		rows, err := s.q.ListAssignmentsNextPage(ctx, gen.ListAssignmentsNextPageParams{
			CreatedAt: cursor.At, ID: cursor.ID, Limit: int32(limit + 1),
		})
		if err != nil {
			return nil, "", 0, err
		}
		for _, r := range rows {
			out = append(out, assignmentFromRow(r.ID, r.GroupID, r.Name, r.ApplicationID, r.EnvironmentID, r.RevisionID, r.Status, r.CreatedAt))
		}
	}
	items, next, err := trimPage(out, limit, func(a Assignment) (time.Time, string) {
		at, _ := time.Parse(time.RFC3339, a.CreatedAt)
		return at, a.ID
	})
	return items, next, total, err
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
