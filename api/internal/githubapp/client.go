package githubapp

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

const apiVersion = "2022-11-28"

type Client struct {
	appID      int64
	slug       string
	privateKey *rsa.PrivateKey
	baseURL    string
	http       *http.Client
	now        func() time.Time
}

type installationResponse struct {
	ID                  int64  `json:"id"`
	RepositorySelection string `json:"repository_selection"`
	Account             struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
}

type accessTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type repositoryResponse struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
}

func NewFromFile(appID int64, slug, keyPath, baseURL string) (*Client, error) {
	pem, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read GitHub App private key: %w", err)
	}
	return New(appID, slug, pem, baseURL, nil)
}

func New(appID int64, slug string, privateKeyPEM []byte, baseURL string, httpClient *http.Client) (*Client, error) {
	if appID <= 0 || strings.TrimSpace(slug) == "" {
		return nil, domain.ErrGitHubAppNotConfigured
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse GitHub App private key: %w", err)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{appID: appID, slug: strings.TrimSpace(slug), privateKey: key, baseURL: strings.TrimRight(baseURL, "/"), http: httpClient, now: time.Now}, nil
}

func (c *Client) InstallURL(state string) string {
	values := url.Values{}
	values.Set("state", state)
	return "https://github.com/apps/" + url.PathEscape(c.slug) + "/installations/new?" + values.Encode()
}

func (c *Client) Installation(ctx context.Context, installationID int64) (domain.GitHubInstallation, error) {
	token, err := c.appJWT()
	if err != nil {
		return domain.GitHubInstallation{}, err
	}
	var response installationResponse
	if err := c.request(ctx, http.MethodGet, "/app/installations/"+strconv.FormatInt(installationID, 10), token, nil, &response); err != nil {
		return domain.GitHubInstallation{}, fmt.Errorf("get GitHub App installation: %w", err)
	}
	if response.ID != installationID || response.Account.Login == "" {
		return domain.GitHubInstallation{}, domain.ErrGitHubInstallationInvalid
	}
	return domain.GitHubInstallation{
		InstallationID:      response.ID,
		AccountLogin:        response.Account.Login,
		AccountType:         response.Account.Type,
		RepositorySelection: response.RepositorySelection,
	}, nil
}

func (c *Client) ListRepositories(ctx context.Context, installationID int64) ([]domain.GitHubRepository, error) {
	token, err := c.InstallationToken(ctx, installationID, 0)
	if err != nil {
		return nil, err
	}
	result := make([]domain.GitHubRepository, 0)
	for page := 1; ; page++ {
		var response struct {
			Repositories []repositoryResponse `json:"repositories"`
		}
		path := "/installation/repositories?per_page=100&page=" + strconv.Itoa(page)
		if err := c.request(ctx, http.MethodGet, path, token, nil, &response); err != nil {
			return nil, fmt.Errorf("list GitHub App repositories: %w", err)
		}
		for _, repository := range response.Repositories {
			result = append(result, domain.GitHubRepository{
				ID: repository.ID, FullName: repository.FullName, Private: repository.Private, DefaultBranch: repository.DefaultBranch,
			})
		}
		if len(response.Repositories) < 100 {
			break
		}
	}
	return result, nil
}

func (c *Client) InstallationToken(ctx context.Context, installationID, repositoryID int64) (string, error) {
	appToken, err := c.appJWT()
	if err != nil {
		return "", err
	}
	var body any
	if repositoryID > 0 {
		body = struct {
			RepositoryIDs []int64 `json:"repository_ids"`
		}{RepositoryIDs: []int64{repositoryID}}
	}
	var response accessTokenResponse
	path := "/app/installations/" + strconv.FormatInt(installationID, 10) + "/access_tokens"
	if err := c.request(ctx, http.MethodPost, path, appToken, body, &response); err != nil {
		return "", fmt.Errorf("create GitHub installation token: %w", err)
	}
	if response.Token == "" {
		return "", fmt.Errorf("create GitHub installation token: empty token")
	}
	return response.Token, nil
}

func (c *Client) appJWT() (string, error) {
	now := c.now()
	claims := jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(c.appID, 10),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(c.privateKey)
}

func (c *Client) request(ctx context.Context, method, path, token string, body, destination any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		if response.StatusCode == http.StatusNotFound {
			return domain.ErrGitHubInstallationInvalid
		}
		return fmt.Errorf("GitHub API returned %d: %s", response.StatusCode, strings.TrimSpace(string(limited)))
	}
	if destination == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode GitHub API response: %w", err)
	}
	return nil
}
