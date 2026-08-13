package database

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// tsPtr converts a nullable pgx timestamp into the package's *time.Time
// representation used by the public row structs.
func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// pgTS converts a *time.Time into a nullable pgx timestamp parameter.
func pgTS(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
