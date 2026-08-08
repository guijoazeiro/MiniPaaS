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
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	PublicURL string `json:"public_url,omitempty"`
}

type Deployment struct {
	ID         string `json:"id"`
	AppID      string `json:"app_id"`
	ImageTag   string `json:"image_tag"`
	Status     string `json:"status"`
	Port       int    `json:"port,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
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

func (c *Client) GetDeployment(app, id string) (*Deployment, error) {
	var out Deployment
	if err := c.doJSON(http.MethodGet, "/apps/"+app+"/deployments/"+id, nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
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
	if out == nil || resp.ContentLength == 0 {
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
	body, _ := json.Marshal(map[string]string{"deployment_id": deploymentID})
	var out Deployment
	if err := c.doJSON(http.MethodPost, "/apps/"+app+"/rollback", bytes.NewReader(body), "application/json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
