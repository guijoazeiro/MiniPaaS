package service

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/crypto"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

var envKeyRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type EnvService struct {
	envs   store.EnvStore
	cipher *crypto.Cipher
}

func NewEnvService(envs store.EnvStore, cipher *crypto.Cipher) *EnvService {
	return &EnvService{envs: envs, cipher: cipher}
}

func (s *EnvService) Set(ctx context.Context, appID uuid.UUID, key, value string) error {
	if !envKeyRE.MatchString(key) {
		return domain.ErrEnvKeyInvalid
	}
	ct, nonce, err := s.cipher.Encrypt([]byte(value))
	if err != nil {
		return fmt.Errorf("env.Set: %w", err)
	}
	return s.envs.Upsert(ctx, appID, key, ct, nonce)
}

func (s *EnvService) List(ctx context.Context, appID uuid.UUID) ([]domain.EnvVarKey, error) {
	return s.envs.ListKeys(ctx, appID)
}

func (s *EnvService) Delete(ctx context.Context, appID uuid.UUID, key string) error {
	return s.envs.Delete(ctx, appID, key)
}

func (s *EnvService) Decrypted(ctx context.Context, appID uuid.UUID) (map[string]string, error) {
	records, err := s.envs.ListRecords(ctx, appID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(records))
	for _, r := range records {
		pt, err := s.cipher.Decrypt(r.Value, r.Nonce)
		if err != nil {
			return nil, fmt.Errorf("env.Decrypted: %s: %w", r.Key, err)
		}
		out[r.Key] = string(pt)
	}
	return out, nil
}
