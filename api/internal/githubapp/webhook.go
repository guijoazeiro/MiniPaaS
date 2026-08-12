package githubapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

func VerifyWebhookSignature(secret string, payload []byte, signature string) error {
	if strings.TrimSpace(secret) == "" {
		return domain.ErrGitHubWebhookNotConfigured
	}
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return domain.ErrGitHubWebhookSignatureInvalid
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, prefix))
	if err != nil {
		return domain.ErrGitHubWebhookSignatureInvalid
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return domain.ErrGitHubWebhookSignatureInvalid
	}
	return nil
}
