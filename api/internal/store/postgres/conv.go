package postgres

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func uuidToPG(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func pgToUUID(v pgtype.UUID) uuid.UUID {
	if !v.Valid {
		return uuid.Nil
	}
	return uuid.UUID(v.Bytes)
}

func pgText(s pgtype.Text) string {
	if !s.Valid {
		return ""
	}
	return s.String
}

func pgInt4(v pgtype.Int4) int {
	if !v.Valid {
		return 0
	}
	return int(v.Int32)
}
