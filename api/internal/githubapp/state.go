package githubapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type StateSigner struct {
	secret []byte
	now    func() time.Time
}

type stateClaims struct {
	AppName string `json:"app_name"`
	jwt.RegisteredClaims
}

func NewStateSigner(secret []byte) *StateSigner {
	return &StateSigner{secret: secret, now: time.Now}
}

func (s *StateSigner) Sign(appName string) (string, error) {
	if len(s.secret) == 0 || strings.TrimSpace(appName) == "" {
		return "", domain.ErrGitHubInstallationInvalid
	}
	now := s.now()
	claims := stateClaims{
		AppName: appName,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "minipaas-github-app",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *StateSigner) Verify(raw string) (string, error) {
	claims := &stateClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	}, jwt.WithIssuer("minipaas-github-app"), jwt.WithTimeFunc(s.now))
	if err != nil || !token.Valid || strings.TrimSpace(claims.AppName) == "" {
		return "", domain.ErrGitHubInstallationInvalid
	}
	return claims.AppName, nil
}
