package githubapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type StateSigner struct {
	secret []byte
	now    func() time.Time
}

type stateClaims struct {
	AppName string `json:"app_name"`
	UserID  string `json:"user_id"`
	jwt.RegisteredClaims
}

func NewStateSigner(secret []byte) *StateSigner {
	return &StateSigner{secret: secret, now: time.Now}
}

func (s *StateSigner) Sign(appName string, userID uuid.UUID) (string, error) {
	if len(s.secret) == 0 || strings.TrimSpace(appName) == "" || userID == uuid.Nil {
		return "", domain.ErrGitHubInstallationInvalid
	}
	now := s.now()
	claims := stateClaims{
		AppName: appName,
		UserID:  userID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "minipaas-github-app",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *StateSigner) Verify(raw string) (string, uuid.UUID, error) {
	claims := &stateClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	}, jwt.WithIssuer("minipaas-github-app"), jwt.WithTimeFunc(s.now))
	userID, userErr := uuid.Parse(claims.UserID)
	if err != nil || userErr != nil || token == nil || !token.Valid || strings.TrimSpace(claims.AppName) == "" || userID == uuid.Nil {
		return "", uuid.Nil, domain.ErrGitHubInstallationInvalid
	}
	return claims.AppName, userID, nil
}
