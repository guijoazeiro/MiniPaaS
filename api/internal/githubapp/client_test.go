package githubapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func testPrivateKey(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func TestClientInstallationTokenAndRepositories(t *testing.T) {
	var appToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-GitHub-Api-Version") == "" {
			t.Error("missing GitHub API version header")
		}
		switch r.URL.Path {
		case "/app/installations/42":
			appToken = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			parsed, _, err := jwt.NewParser().ParseUnverified(appToken, jwt.MapClaims{})
			if err != nil || parsed == nil {
				t.Errorf("invalid app JWT: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":42,"repository_selection":"selected","account":{"login":"acme","type":"Organization"}}`))
		case "/app/installations/42/access_tokens":
			if !strings.Contains(r.Header.Get("Authorization"), "Bearer ") {
				t.Error("missing app authentication")
			}
			_, _ = w.Write([]byte(`{"token":"installation-secret","expires_at":"2030-01-01T00:00:00Z"}`))
		case "/installation/repositories":
			if r.Header.Get("Authorization") != "Bearer installation-secret" {
				t.Errorf("unexpected installation authentication: %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"repositories":[{"id":99,"full_name":"acme/private-api","private":true,"default_branch":"trunk"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(123, "mini-paas", testPrivateKey(t), server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	installation, err := client.Installation(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if installation.AccountLogin != "acme" || appToken == "" {
		t.Fatalf("installation = %+v, appToken present = %v", installation, appToken != "")
	}
	repositories, err := client.ListRepositories(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 || repositories[0].ID != 99 || !repositories[0].Private {
		t.Fatalf("repositories = %+v", repositories)
	}
}

func TestStateSignerRejectsExpiredAndTamperedState(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	signer := NewStateSigner([]byte("state-secret"))
	signer.now = func() time.Time { return now }
	userID := uuid.New()
	state, err := signer.Sign("private-api", userID)
	if err != nil {
		t.Fatal(err)
	}
	appName, gotUserID, err := signer.Verify(state)
	if err != nil || appName != "private-api" || gotUserID != userID {
		t.Fatalf("Verify() = %q/%s, %v", appName, gotUserID, err)
	}
	if _, _, err := signer.Verify(state + "x"); err == nil {
		t.Fatal("tampered state was accepted")
	}
	signer.now = func() time.Time { return now.Add(11 * time.Minute) }
	if _, _, err := signer.Verify(state); err == nil {
		t.Fatal("expired state was accepted")
	}
}

func TestStateSignerSupportsAccountTarget(t *testing.T) {
	signer := NewStateSigner([]byte("state-secret"))
	userID := uuid.New()
	state, err := signer.SignAccount(userID)
	if err != nil {
		t.Fatal(err)
	}
	appName, gotUserID, target, err := signer.VerifyTarget(state)
	if err != nil || appName != "" || gotUserID != userID || target != "account" {
		t.Fatalf("VerifyTarget() = %q/%s/%q, %v", appName, gotUserID, target, err)
	}
}
