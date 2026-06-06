package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// uuidFromString parses a textual UUID into pgtype.UUID. Returns ErrNotFound
// when the input does not look like a UUID — callers can treat unparseable
// IDs the same as missing rows.
func uuidFromString(s string) (pgtype.UUID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("repository: parse uuid %q: %w", s, err)
	}
	return pgtype.UUID{Bytes: u, Valid: true}, nil
}

// uuidToString formats pgtype.UUID as canonical hyphenated UUID.
// Returns the empty string when the value is not valid.
func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

// tsFromTime converts time.Time to pgtype.Timestamptz with Valid=true.
func tsFromTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// timeFromTs returns the underlying time.Time, or the zero value when invalid.
func timeFromTs(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// isUniqueViolation reports whether err is a Postgres unique_violation (23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
