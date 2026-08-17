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
	Target  string `json:"target"`
	jwt.RegisteredClaims
}

func NewStateSigner(secret []byte) *StateSigner {
	return &StateSigner{secret: secret, now: time.Now}
}

func (s *StateSigner) Sign(appName string, userID uuid.UUID) (string, error) {
	return s.sign(appName, userID, "app")
}

func (s *StateSigner) SignAccount(userID uuid.UUID) (string, error) {
	return s.sign("", userID, "account")
}

func (s *StateSigner) sign(appName string, userID uuid.UUID, target string) (string, error) {
	if len(s.secret) == 0 || userID == uuid.Nil || (target == "app" && strings.TrimSpace(appName) == "") || (target != "app" && target != "account") {
		return "", domain.ErrGitHubInstallationInvalid
	}
	now := s.now()
	claims := stateClaims{
		AppName: appName,
		UserID:  userID.String(),
		Target:  target,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "minipaas-github-app",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *StateSigner) Verify(raw string) (string, uuid.UUID, error) {
	appName, userID, _, err := s.VerifyTarget(raw)
	return appName, userID, err
}

func (s *StateSigner) VerifyTarget(raw string) (string, uuid.UUID, string, error) {
	claims := &stateClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	}, jwt.WithIssuer("minipaas-github-app"), jwt.WithTimeFunc(s.now))
	userID, userErr := uuid.Parse(claims.UserID)
	if err != nil || userErr != nil || token == nil || !token.Valid || userID == uuid.Nil {
		return "", uuid.Nil, "", domain.ErrGitHubInstallationInvalid
	}
	if claims.Target == "" && strings.TrimSpace(claims.AppName) != "" {
		// Accept states issued before the account-installation flow was added.
		claims.Target = "app"
	}
	if (claims.Target != "app" && claims.Target != "account") || (claims.Target == "app" && strings.TrimSpace(claims.AppName) == "") || (claims.Target == "account" && strings.TrimSpace(claims.AppName) != "") {
		return "", uuid.Nil, "", domain.ErrGitHubInstallationInvalid
	}
	return claims.AppName, userID, claims.Target, nil
}
