package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserStore struct {
	users  map[string]domain.User
	countN int64
	create func(username, hash string) (domain.User, error)
	get    func(username string) (domain.User, error)
}

func (f *fakeUserStore) Create(_ context.Context, u, h string) (domain.User, error) {
	if f.create != nil {
		return f.create(u, h)
	}
	user := domain.User{ID: uuid.New(), Username: u, PasswordHash: h}
	if f.users == nil {
		f.users = map[string]domain.User{}
	}
	f.users[u] = user
	f.countN++
	return user, nil
}
func (f *fakeUserStore) GetByUsername(_ context.Context, u string) (domain.User, error) {
	if f.get != nil {
		return f.get(u)
	}
	if user, ok := f.users[u]; ok {
		return user, nil
	}
	return domain.User{}, domain.ErrInvalidCredentials
}

func (f *fakeUserStore) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	for _, user := range f.users {
		if user.ID == id {
			return user, nil
		}
	}
	return domain.User{}, domain.ErrUserNotFound
}

func (f *fakeUserStore) UpdateUsername(_ context.Context, id uuid.UUID, username string) (domain.User, error) {
	user, err := f.GetByID(context.Background(), id)
	if err != nil {
		return domain.User{}, err
	}
	if existing, ok := f.users[username]; ok && existing.ID != id {
		return domain.User{}, domain.ErrUsernameTaken
	}
	delete(f.users, user.Username)
	user.Username = username
	f.users[username] = user
	return user, nil
}

func (f *fakeUserStore) UpdatePassword(_ context.Context, id uuid.UUID, passwordHash string) error {
	user, err := f.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	user.PasswordHash = passwordHash
	f.users[user.Username] = user
	return nil
}

func TestLoginPreservesStoreErrors(t *testing.T) {
	svc, users := newAuthSvc(t)
	dbErr := errors.New("database unavailable")
	users.get = func(string) (domain.User, error) { return domain.User{}, dbErr }

	_, _, err := svc.Login(context.Background(), "alice", "hunter2")
	if !errors.Is(err, dbErr) {
		t.Fatalf("Login() error = %v, want wrapped %v", err, dbErr)
	}
}
func (f *fakeUserStore) Count(_ context.Context) (int64, error) { return f.countN, nil }

func newAuthSvc(t *testing.T) (*AuthService, *fakeUserStore) {
	t.Helper()
	users := &fakeUserStore{}
	svc := NewAuthService(users, []byte("test-secret"), time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	hash, _ := bcrypt.GenerateFromPassword([]byte("hunter2"), bcrypt.MinCost)
	users.users = map[string]domain.User{
		"alice": {ID: uuid.New(), Username: "alice", PasswordHash: string(hash)},
	}
	users.countN = 1
	return svc, users
}

func TestLogin(t *testing.T) {
	svc, _ := newAuthSvc(t)

	tests := []struct {
		name    string
		user    string
		pass    string
		wantErr error
	}{
		{"happy", "alice", "hunter2", nil},
		{"wrong password", "alice", "wrong", domain.ErrInvalidCredentials},
		{"unknown user", "bob", "hunter2", domain.ErrInvalidCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok, exp, err := svc.Login(context.Background(), tt.user, tt.pass)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if tok == "" {
					t.Fatal("empty token on success")
				}
				if exp.Before(time.Now()) {
					t.Fatal("expiry in the past")
				}
			}
		})
	}
}

func TestParseTokenRoundtrip(t *testing.T) {
	svc, users := newAuthSvc(t)
	alice := users.users["alice"]

	tok, _, err := svc.Login(context.Background(), "alice", "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	id, name, err := svc.ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if id != alice.ID {
		t.Errorf("id = %s, want %s", id, alice.ID)
	}
	if name != "alice" {
		t.Errorf("name = %s, want alice", name)
	}
}

func TestParseTokenRejectsDifferentSigningKey(t *testing.T) {
	svc, _ := newAuthSvc(t)
	tok, _, _ := svc.Login(context.Background(), "alice", "hunter2")
	otherSvc := NewAuthService(&fakeUserStore{}, []byte("different-secret"), time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, _, err := otherSvc.ParseToken(tok); err == nil {
		t.Fatal("expected error parsing token signed with a different key")
	}
}

func TestSeedAdminIdempotent(t *testing.T) {
	users := &fakeUserStore{}
	svc := NewAuthService(users, []byte("s"), time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := svc.SeedAdmin(context.Background(), "root", "pw"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SeedAdmin(context.Background(), "root", "pw"); err != nil {
		t.Fatal(err)
	}
	if users.countN != 1 {
		t.Errorf("users created = %d, want 1", users.countN)
	}
}

func TestRegisterAndUpdateAccount(t *testing.T) {
	svc, users := newAuthSvc(t)
	created, err := svc.Register(context.Background(), "bob", "strong-pass")
	if err != nil {
		t.Fatal(err)
	}
	if created.Username != "bob" || created.PasswordHash == "" {
		t.Fatalf("created user = %+v", created)
	}

	updated, err := svc.UpdateUsername(context.Background(), created.ID, "robert")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Username != "robert" {
		t.Fatalf("updated username = %q", updated.Username)
	}
	if err := svc.UpdatePassword(context.Background(), created.ID, "strong-pass", "stronger-pass"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(context.Background(), "robert", "stronger-pass"); err != nil {
		t.Fatalf("login with updated credentials: %v", err)
	}
	if _, _, err := svc.Login(context.Background(), "robert", "strong-pass"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("old password error = %v", err)
	}
	if _, ok := users.users["robert"]; !ok {
		t.Fatal("updated user was not persisted in fake store")
	}
}

func TestRegisterValidatesUsernameAndPassword(t *testing.T) {
	svc, _ := newAuthSvc(t)
	if _, err := svc.Register(context.Background(), "ab", "strong-pass"); !errors.Is(err, domain.ErrUsernameInvalid) {
		t.Fatalf("short username error = %v", err)
	}
	if _, err := svc.Register(context.Background(), "bob", "short"); !errors.Is(err, domain.ErrPasswordWeak) {
		t.Fatalf("short password error = %v", err)
	}
}
