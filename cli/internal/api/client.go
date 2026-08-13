package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type Client struct {
	base  string
	token string
	http  *http.Client
}

func New(base, token string) *Client {
	return &Client{
		base:  base,
		token: token,
		http:  &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *Client) Host() string { return c.base }

type App struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	ContainerState string `json:"container_state,omitempty"`
	PublicURL      string `json:"public_url,omitempty"`
}

type Deployment struct {
	ID               string `json:"id"`
	AppID            string `json:"app_id"`
	ImageTag         string `json:"image_tag"`
	Status           string `json:"status"`
	Port             int    `json:"port,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	DurationMs       int    `json:"duration_ms,omitempty"`
	SourceType       string `json:"source_type,omitempty"`
	Repository       string `json:"repository,omitempty"`
	Branch           string `json:"branch,omitempty"`
	CommitSHA        string `json:"commit_sha,omitempty"`
	CommitAuthor     string `json:"commit_author,omitempty"`
	CommitMessage    string `json:"commit_message,omitempty"`
	TriggerType      string `json:"trigger_type,omitempty"`
	GitHubDeliveryID string `json:"github_delivery_id,omitempty"`
	Attempt          int    `json:"attempt,omitempty"`
	RetryOf          string `json:"retry_of,omitempty"`
	CancelRequested  bool   `json:"cancel_requested,omitempty"`
}

type DeploymentLog struct {
	ID           int64  `json:"id"`
	DeploymentID string `json:"deployment_id"`
	Stage        string `json:"stage"`
	Stream       string `json:"stream"`
	Message      string `json:"message"`
	CreatedAt    string `json:"created_at"`
}

type GitSource struct {
	AppID                string `json:"app_id"`
	Repository           string `json:"repository"`
	Branch               string `json:"branch"`
	BuildContext         string `json:"build_context"`
	DockerfilePath       string `json:"dockerfile_path"`
	AccessMode           string `json:"access_mode"`
	GitHubInstallationID int64  `json:"github_installation_id,omitempty"`
	GitHubRepositoryID   int64  `json:"github_repository_id,omitempty"`
	Private              bool   `json:"private"`
	AutoDeploy           bool   `json:"auto_deploy"`
}

type GitHubInstallation struct {
	InstallationID      int64  `json:"installation_id"`
	AccountLogin        string `json:"account_login"`
	AccountType         string `json:"account_type"`
	RepositorySelection string `json:"repository_selection"`
}

type GitHubRepository struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
}

func (c *Client) ListDeployments(app string) ([]Deployment, error) {
	var out []Deployment
	if err := c.doJSON(http.MethodGet, "/apps/"+app+"/deployments?limit=5", nil, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateApp(name string) (*App, error) {
	body, _ := json.Marshal(map[string]string{"name": name})
	var out App
	if err := c.doJSON(http.MethodPost, "/apps", bytes.NewReader(body), "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListApps() ([]App, error) {
	var out []App
	if err := c.doJSON(http.MethodGet, "/apps", nil, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetApp(name string) (*App, error) {
	var out App
	if err := c.doJSON(http.MethodGet, "/apps/"+name, nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Deploy(app string, tar io.Reader) (*Deployment, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer mw.Close()
		part, err := mw.CreateFormFile("source", "context.tar")
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, tar); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
	}()

	req, err := http.NewRequest(http.MethodPost, c.base+"/apps/"+app+"/deployments", pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.authHeader(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, httpErr(resp)
	}
	var out Deployment
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &out, nil
}

func (c *Client) ConfigureGitSource(app string, source GitSource) (*GitSource, error) {
	body, _ := json.Marshal(source)
	var out GitSource
	if err := c.doJSON(http.MethodPut, "/apps/"+app+"/source/git", bytes.NewReader(body), "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ConfigureGitHubAppSource(app string, installationID, repositoryID int64, branch, buildContext, dockerfilePath string) (*GitSource, error) {
	body, _ := json.Marshal(map[string]any{
		"installation_id": installationID,
		"repository_id":   repositoryID,
		"branch":          branch,
		"build_context":   buildContext,
		"dockerfile_path": dockerfilePath,
	})
	var out GitSource
	if err := c.doJSON(http.MethodPut, "/apps/"+app+"/source/github-app", bytes.NewReader(body), "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListGitHubInstallations() ([]GitHubInstallation, error) {
	var out []GitHubInstallation
	if err := c.doJSON(http.MethodGet, "/integrations/github/installations", nil, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListGitHubRepositories(installationID int64) ([]GitHubRepository, error) {
	var out []GitHubRepository
	path := fmt.Sprintf("/integrations/github/installations/%d/repositories", installationID)
	if err := c.doJSON(http.MethodGet, path, nil, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetGitSource(app string) (*GitSource, error) {
	var out GitSource
	if err := c.doJSON(http.MethodGet, "/apps/"+app+"/source/git", nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteGitSource(app string) error {
	return c.doJSON(http.MethodDelete, "/apps/"+app+"/source/git", nil, "", nil)
}

func (c *Client) SetGitAutoDeploy(app string, enabled bool) (*GitSource, error) {
	body, _ := json.Marshal(map[string]bool{"enabled": enabled})
	var out GitSource
	if err := c.doJSON(http.MethodPatch, "/apps/"+app+"/source/git/auto-deploy", bytes.NewReader(body), "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeployGit(app, branch string) (*Deployment, error) {
	body, _ := json.Marshal(map[string]string{"branch": branch})
	var out Deployment
	if err := c.doJSON(http.MethodPost, "/apps/"+app+"/deployments/git", bytes.NewReader(body), "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDeployment(app, id string) (*Deployment, error) {
	var out Deployment
	if err := c.doJSON(http.MethodGet, "/apps/"+app+"/deployments/"+id, nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RetryDeployment(app, id string) (*Deployment, error) {
	var out Deployment
	if err := c.doJSON(http.MethodPost, "/apps/"+app+"/deployments/"+id+"/retry", nil, "", &out); err != nil { return nil, err }
	return &out, nil
}

func (c *Client) CancelDeployment(app, id string) (*Deployment, error) {
	var out Deployment
	if err := c.doJSON(http.MethodPost, "/apps/"+app+"/deployments/"+id+"/cancel", nil, "", &out); err != nil { return nil, err }
	return &out, nil
}

func (c *Client) ListDeploymentLogs(app, id string, after, limit int64) ([]DeploymentLog, error) {
	path := fmt.Sprintf("/apps/%s/deployments/%s/logs?after=%d&limit=%d", app, id, after, limit)
	var out []DeploymentLog
	if err := c.doJSON(http.MethodGet, path, nil, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) doJSON(method, path string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.authHeader(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return httpErr(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func httpErr(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
	return fmt.Errorf("http %d: %s", resp.StatusCode, bytes.TrimSpace(b))
}

func (c *Client) authHeader(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

type LoginResp struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

func (c *Client) Login(username, password string) (*LoginResp, error) {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	var out LoginResp
	if err := c.doJSON(http.MethodPost, "/auth/login", bytes.NewReader(body), "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type EnvKey struct {
	Key       string `json:"key"`
	UpdatedAt string `json:"updated_at"`
}

func (c *Client) ListEnv(app string) ([]EnvKey, error) {
	var out []EnvKey
	if err := c.doJSON(http.MethodGet, "/apps/"+app+"/env", nil, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) SetEnv(app, key, value string) error {
	body, _ := json.Marshal(map[string]string{"value": value})
	return c.doJSON(http.MethodPut, "/apps/"+app+"/env/"+key, bytes.NewReader(body), "application/json", nil)
}

func (c *Client) UnsetEnv(app, key string) error {
	return c.doJSON(http.MethodDelete, "/apps/"+app+"/env/"+key, nil, "", nil)
}

func (c *Client) Rollback(app, deploymentID string) (*Deployment, error) {
	body, _ := json.Marshal(map[string]string{"deployment_id": deploymentID, "triggered_by": "cli"})
	var out Deployment
	if err := c.doJSON(http.MethodPost, "/apps/"+app+"/rollback", bytes.NewReader(body), "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
