package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
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

var usernameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{2,63}$`)

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
	return s.issueToken(u)
}

func (s *AuthService) issueToken(u domain.User) (token string, expiresAt time.Time, err error) {
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

func (s *AuthService) Register(ctx context.Context, username, password string) (domain.User, error) {
	username = strings.TrimSpace(username)
	if !usernameRE.MatchString(username) {
		return domain.User{}, domain.ErrUsernameInvalid
	}
	if len([]rune(password)) < 8 {
		return domain.User{}, domain.ErrPasswordWeak
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("auth.Register: hash: %w", err)
	}
	user, err := s.users.Create(ctx, username, string(hash))
	if err != nil {
		return domain.User{}, fmt.Errorf("auth.Register: %w", err)
	}
	return user, nil
}

func (s *AuthService) Profile(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("auth.Profile: %w", err)
	}
	return user, nil
}

func (s *AuthService) UpdateUsername(ctx context.Context, userID uuid.UUID, username string) (domain.User, error) {
	username = strings.TrimSpace(username)
	if !usernameRE.MatchString(username) {
		return domain.User{}, domain.ErrUsernameInvalid
	}
	user, err := s.users.UpdateUsername(ctx, userID, username)
	if err != nil {
		return domain.User{}, fmt.Errorf("auth.UpdateUsername: %w", err)
	}
	return user, nil
}

func (s *AuthService) UpdatePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	if len([]rune(newPassword)) < 8 {
		return domain.ErrPasswordWeak
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("auth.UpdatePassword: get user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return domain.ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("auth.UpdatePassword: hash: %w", err)
	}
	if err := s.users.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return fmt.Errorf("auth.UpdatePassword: save: %w", err)
	}
	return nil
}

func (s *AuthService) RefreshToken(ctx context.Context, userID uuid.UUID) (string, time.Time, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth.RefreshToken: get user: %w", err)
	}
	return s.issueToken(user)
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
