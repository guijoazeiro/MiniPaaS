package githubapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

func TestVerifyWebhookSignature(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	_, _ = mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if err := VerifyWebhookSignature("webhook-secret", payload, signature); err != nil {
		t.Fatalf("VerifyWebhookSignature() error = %v", err)
	}
	if err := VerifyWebhookSignature("webhook-secret", append(payload, 'x'), signature); !errors.Is(err, domain.ErrGitHubWebhookSignatureInvalid) {
		t.Fatalf("tampered payload error = %v", err)
	}
	if err := VerifyWebhookSignature("", payload, signature); !errors.Is(err, domain.ErrGitHubWebhookNotConfigured) {
		t.Fatalf("empty secret error = %v", err)
	}
}
