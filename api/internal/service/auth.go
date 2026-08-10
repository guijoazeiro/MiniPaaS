package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users     store.UserStore
	jwtSecret []byte
	tokenTTL  time.Duration
	log       *slog.Logger
}

var dummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("minipaas-dummy-password"), bcrypt.DefaultCost)
	if err != nil {
		panic("generate dummy bcrypt hash: " + err.Error())
	}
	return hash
}()

func NewAuthService(users store.UserStore, jwtSecret []byte, tokenTTL time.Duration, log *slog.Logger) *AuthService {
	return &AuthService{users: users, jwtSecret: jwtSecret, tokenTTL: tokenTTL, log: log}
}

func (s *AuthService) Login(ctx context.Context, username, password string) (token string, expiresAt time.Time, err error) {
	u, err := s.users.GetByUsername(ctx, username)
	userExists := true
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			userExists = false
		} else {
			return "", time.Time{}, fmt.Errorf("auth.Login: get user: %w", err)
		}
	}
	hash := dummyPasswordHash
	if userExists {
		hash = []byte(u.PasswordHash)
	}
	passwordErr := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if !userExists || passwordErr != nil {
		return "", time.Time{}, domain.ErrInvalidCredentials
	}
	expiresAt = time.Now().Add(s.tokenTTL)
	claims := jwt.MapClaims{
		"sub": u.ID.String(),
		"usr": u.Username,
		"exp": expiresAt.Unix(),
		"iat": time.Now().Unix(),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth.Login: sign: %w", err)
	}
	return tok, expiresAt, nil
}

func (s *AuthService) ParseToken(raw string) (uuid.UUID, string, error) {
	tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !tok.Valid {
		return uuid.Nil, "", domain.ErrUnauthorized
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, "", domain.ErrUnauthorized
	}
	sub, _ := claims["sub"].(string)
	usr, _ := claims["usr"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, "", domain.ErrUnauthorized
	}
	return id, usr, nil
}

func (s *AuthService) SeedAdmin(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return nil
	}
	n, err := s.users.Count(ctx)
	if err != nil {
		return fmt.Errorf("auth.SeedAdmin: count: %w", err)
	}
	if n > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("auth.SeedAdmin: hash: %w", err)
	}
	if _, err := s.users.Create(ctx, username, string(hash)); err != nil {
		return fmt.Errorf("auth.SeedAdmin: create: %w", err)
	}
	s.log.Info("seeded admin user", "username", username)
	return nil
}
