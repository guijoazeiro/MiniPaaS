package postgres

import (
	"testing"

	"github.com/google/uuid"
)

func TestNullableUUID(t *testing.T) {
	if got := nullableUUID(uuid.Nil); got.Valid {
		t.Fatal("uuid.Nil must be encoded as SQL NULL")
	}

	id := uuid.New()
	got := nullableUUID(id)
	if !got.Valid || uuid.UUID(got.Bytes) != id {
		t.Fatalf("nullableUUID(%s) = %+v, want valid UUID", id, got)
	}
}
