package sourcegit

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

func TestNormalizeRepository(t *testing.T) {
	tests := map[string]string{
		"owner/repo":                        "owner/repo",
		"https://github.com/owner/repo.git": "owner/repo",
		"git@github.com:owner/repo.git":     "owner/repo",
	}
	for input, want := range tests {
		got, err := NormalizeRepository(input)
		if err != nil || got != want {
			t.Fatalf("NormalizeRepository(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"https://example.com/owner/repo", "github.com/owner/repo", "owner/repo/extra", "owner/../repo", "https://user:pass@github.com/owner/repo"} {
		if _, err := NormalizeRepository(input); !errors.Is(err, domain.ErrGitRepositoryInvalid) {
			t.Fatalf("NormalizeRepository(%q) error = %v", input, err)
		}
	}
}

func TestNormalizeBranchAndPath(t *testing.T) {
	if got, err := NormalizeBranch(""); err != nil || got != "main" {
		t.Fatalf("default branch = %q, %v", got, err)
	}
	if got, err := NormalizeBranch("release/v1"); err != nil || got != "release/v1" {
		t.Fatalf("branch = %q, %v", got, err)
	}
	for _, branch := range []string{"../main", "feature branch", "bad~ref", "-danger"} {
		if _, err := NormalizeBranch(branch); !errors.Is(err, domain.ErrGitRefInvalid) {
			t.Fatalf("branch %q error = %v", branch, err)
		}
	}
	for _, path := range []string{"../secret", `..\secret`, "folder/../../secret", "/absolute"} {
		if _, err := NormalizeRelativePath(path, "."); !errors.Is(err, domain.ErrGitPathInvalid) {
			t.Fatalf("path %q error = %v", path, err)
		}
	}
	if got, err := NormalizeRelativePath("services/api", "."); err != nil || got != "services/api" {
		t.Fatalf("path = %q, %v", got, err)
	}
}

func TestPackDirectoryExcludesGitAndLimitsSize(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM scratch"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	var archive bytes.Buffer
	if err := packDirectory(root, &archive, 1024); err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(bytes.NewReader(archive.Bytes()))
	var names []string
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, header.Name)
	}
	if len(names) != 1 || names[0] != "Dockerfile" {
		t.Fatalf("archive entries = %v", names)
	}

	if err := packDirectory(root, io.Discard, 2); err == nil {
		t.Fatal("expected repository size error")
	}
}

func TestSecureDockerfileRequiresRegularFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM scratch"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := secureDockerfile(root, "Dockerfile"); err != nil {
		t.Fatal(err)
	}
	if err := secureDockerfile(root, "missing"); !errors.Is(err, domain.ErrDockerfileNotFound) {
		t.Fatalf("missing error = %v", err)
	}
}

func TestPreparePublicGitHubIntegration(t *testing.T) {
	repository := os.Getenv("GITHUB_INTEGRATION_REPOSITORY")
	if repository == "" {
		t.Skip("GITHUB_INTEGRATION_REPOSITORY is required for the network integration test")
	}
	snapshot, err := New(20*1024*1024).Prepare(context.Background(), domain.GitSource{Repository: repository, Branch: "main", BuildContext: ".", DockerfilePath: "Dockerfile"}, "main")
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Source.Close()
	if snapshot.CommitSHA == "" || snapshot.DockerfilePath != "Dockerfile" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	tr := tar.NewReader(snapshot.Source)
	foundDockerfile := false
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == "Dockerfile" {
			foundDockerfile = true
		}
	}
	if !foundDockerfile {
		t.Fatal("prepared archive does not contain Dockerfile")
	}
}
