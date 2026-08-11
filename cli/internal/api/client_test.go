package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoJSONSkipsOnlyNoContentResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := New(srv.URL, "")
	var out struct{}
	if err := client.doJSON(http.MethodDelete, "/", nil, "", &out); err != nil {
		t.Fatalf("doJSON() error = %v", err)
	}
}

func TestGitSourceAndDeployRequests(t *testing.T) {
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut:
			var source GitSource
			if err := json.NewDecoder(r.Body).Decode(&source); err != nil {
				t.Fatal(err)
			}
			if source.Repository != "owner/repo" {
				t.Errorf("repository = %q", source.Repository)
			}
			_ = json.NewEncoder(w).Encode(source)
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(GitSource{Repository: "owner/repo", Branch: "main"})
		case r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(Deployment{ID: "dep", Status: "pending", SourceType: "git"})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	client := New(srv.URL, "token")
	if _, err := client.ConfigureGitSource("app", GitSource{Repository: "owner/repo", Branch: "main"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetGitSource("app"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.DeployGit("app", "main"); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteGitSource("app"); err != nil {
		t.Fatal(err)
	}
	want := []string{"PUT /apps/app/source/git", "GET /apps/app/source/git", "POST /apps/app/deployments/git", "DELETE /apps/app/source/git"}
	if len(requests) != len(want) {
		t.Fatalf("requests = %v", requests)
	}
	for i := range want {
		if requests[i] != want[i] {
			t.Fatalf("requests[%d] = %q, want %q", i, requests[i], want[i])
		}
	}
}
