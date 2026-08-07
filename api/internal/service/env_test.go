package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/crypto"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type fakeEnvStore struct {
	rows map[string]store.EnvVarRecord
}

func newFakeEnvStore() *fakeEnvStore {
	return &fakeEnvStore{rows: map[string]store.EnvVarRecord{}}
}

func k(appID uuid.UUID, key string) string { return appID.String() + "|" + key }

func (f *fakeEnvStore) Upsert(_ context.Context, appID uuid.UUID, key string, v, n []byte) error {
	f.rows[k(appID, key)] = store.EnvVarRecord{Key: key, Value: v, Nonce: n, UpdatedAt: time.Now()}
	return nil
}
func (f *fakeEnvStore) ListKeys(_ context.Context, appID uuid.UUID) ([]domain.EnvVarKey, error) {
	out := []domain.EnvVarKey{}
	for _, r := range f.rows {
		out = append(out, domain.EnvVarKey{Key: r.Key, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}
func (f *fakeEnvStore) ListRecords(_ context.Context, appID uuid.UUID) ([]store.EnvVarRecord, error) {
	out := []store.EnvVarRecord{}
	prefix := appID.String() + "|"
	for kk, r := range f.rows {
		if len(kk) >= len(prefix) && kk[:len(prefix)] == prefix {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeEnvStore) Delete(_ context.Context, appID uuid.UUID, key string) error {
	delete(f.rows, k(appID, key))
	return nil
}

func newEnvSvc(t *testing.T) (*EnvService, *fakeEnvStore) {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	c, err := crypto.New(key)
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeEnvStore()
	return NewEnvService(store, c), store
}

func TestEnvSetDecryptRoundtrip(t *testing.T) {
	svc, store := newEnvSvc(t)
	appID := uuid.New()

	if err := svc.Set(context.Background(), appID, "DATABASE_URL", "postgres://u:p@h/d"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set(context.Background(), appID, "PORT", "8080"); err != nil {
		t.Fatal(err)
	}

	for _, r := range store.rows {
		if bytes.Contains(r.Value, []byte("postgres")) || bytes.Contains(r.Value, []byte("8080")) {
			t.Fatalf("stored value looks like plaintext: %q", r.Value)
		}
	}

	got, err := svc.Decrypted(context.Background(), appID)
	if err != nil {
		t.Fatal(err)
	}
	if got["DATABASE_URL"] != "postgres://u:p@h/d" || got["PORT"] != "8080" {
		t.Fatalf("decrypted mismatch: %+v", got)
	}
}

func TestEnvKeyValidation(t *testing.T) {
	svc, _ := newEnvSvc(t)
	appID := uuid.New()

	bad := []string{"", "0START", "has space", "kebab-case", "punct!"}
	for _, b := range bad {
		if err := svc.Set(context.Background(), appID, b, "x"); !errors.Is(err, domain.ErrEnvKeyInvalid) {
			t.Errorf("Set(%q) err = %v, want ErrEnvKeyInvalid", b, err)
		}
	}
	good := []string{"A", "_x", "MY_VAR_1"}
	for _, g := range good {
		if err := svc.Set(context.Background(), appID, g, "x"); err != nil {
			t.Errorf("Set(%q) err = %v, want nil", g, err)
		}
	}
}

func TestEnvIsolationBetweenApps(t *testing.T) {
	svc, _ := newEnvSvc(t)
	a, b := uuid.New(), uuid.New()

	if err := svc.Set(context.Background(), a, "SECRET", "aaa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set(context.Background(), b, "SECRET", "bbb"); err != nil {
		t.Fatal(err)
	}

	envA, _ := svc.Decrypted(context.Background(), a)
	envB, _ := svc.Decrypted(context.Background(), b)
	if envA["SECRET"] != "aaa" || envB["SECRET"] != "bbb" {
		t.Fatalf("leak: A=%v B=%v", envA, envB)
	}
}
