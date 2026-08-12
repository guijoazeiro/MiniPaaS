package sourcegit

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

var repositoryPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

type Snapshot struct {
	Source         io.ReadCloser
	DockerfilePath string
	CommitSHA      string
	CommitAuthor   string
	CommitMessage  string
	Branch         string
}

type Preparer interface {
	Prepare(ctx context.Context, source domain.GitSource, branch string) (Snapshot, error)
}

type Client struct {
	maxBytes int64
	tokens   InstallationTokenProvider
}

func New(maxBytes int64) *Client { return &Client{maxBytes: maxBytes} }

type InstallationTokenProvider interface {
	InstallationToken(ctx context.Context, installationID, repositoryID int64) (string, error)
}

func NewWithTokenProvider(maxBytes int64, tokens InstallationTokenProvider) *Client {
	return &Client{maxBytes: maxBytes, tokens: tokens}
}

func NormalizeRepository(raw string) (string, error) {
	value := strings.TrimSpace(strings.TrimSuffix(raw, "/"))
	value = strings.TrimSuffix(value, ".git")
	for _, prefix := range []string{"https://github.com/", "http://github.com/", "git@github.com:", "ssh://git@github.com/"} {
		if strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			value = value[len(prefix):]
			break
		}
	}
	parts := strings.Split(value, "/")
	if len(parts) != 2 || !repositoryPart.MatchString(parts[0]) || !repositoryPart.MatchString(parts[1]) || parts[0] == "." || parts[1] == "." {
		return "", domain.ErrGitRepositoryInvalid
	}
	return parts[0] + "/" + parts[1], nil
}

func NormalizeBranch(raw string) (string, error) {
	branch := strings.TrimSpace(raw)
	if branch == "" {
		return "main", nil
	}
	if strings.HasPrefix(branch, "-") || strings.ContainsAny(branch, " ~^:?*[\\") || strings.Contains(branch, "..") || strings.Contains(branch, "@{") || strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, "/") || strings.HasPrefix(branch, "/") || strings.Contains(branch, "//") {
		return "", domain.ErrGitRefInvalid
	}
	return branch, nil
}

func NormalizeRelativePath(raw, fallback string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}
	value = strings.ReplaceAll(value, "\\", "/")
	if filepath.IsAbs(value) || filepath.VolumeName(value) != "" || strings.HasPrefix(value, "/") || value == ".." || strings.HasPrefix(value, "../") {
		return "", domain.ErrGitPathInvalid
	}
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", domain.ErrGitPathInvalid
	}
	return clean, nil
}

func (c *Client) Prepare(ctx context.Context, source domain.GitSource, branch string) (Snapshot, error) {
	repoDir, err := os.MkdirTemp("", "minipaas-git-*")
	if err != nil {
		return Snapshot{}, fmt.Errorf("create clone directory: %w", err)
	}
	fail := func(err error) (Snapshot, error) {
		_ = os.RemoveAll(repoDir)
		return Snapshot{}, err
	}

	branch, err = NormalizeBranch(branch)
	if err != nil {
		return fail(err)
	}
	repository, err := NormalizeRepository(source.Repository)
	if err != nil {
		return fail(err)
	}
	cloneOptions := &git.CloneOptions{
		URL:           "https://github.com/" + repository + ".git",
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Depth:         1,
	}
	if source.AccessMode == domain.GitAccessGitHubApp {
		if c.tokens == nil || source.GitHubInstallationID == nil || source.GitHubRepositoryID == nil {
			return fail(domain.ErrGitHubInstallationInvalid)
		}
		token, tokenErr := c.tokens.InstallationToken(ctx, *source.GitHubInstallationID, *source.GitHubRepositoryID)
		if tokenErr != nil {
			return fail(fmt.Errorf("authorize GitHub repository clone: %w", tokenErr))
		}
		cloneOptions.Auth = &githttp.BasicAuth{Username: "x-access-token", Password: token}
	}
	repo, err := git.PlainCloneContext(ctx, repoDir, false, cloneOptions)
	if err != nil {
		return fail(fmt.Errorf("clone GitHub repository: %w", err))
	}

	contextRel, err := NormalizeRelativePath(source.BuildContext, ".")
	if err != nil {
		return fail(err)
	}
	dockerfileRel, err := NormalizeRelativePath(source.DockerfilePath, "Dockerfile")
	if err != nil {
		return fail(err)
	}
	contextDir, err := secureDirectory(repoDir, contextRel)
	if err != nil {
		return fail(err)
	}
	if err := secureDockerfile(contextDir, dockerfileRel); err != nil {
		return fail(err)
	}

	tarFile, err := os.CreateTemp("", "minipaas-git-build-*.tar")
	if err != nil {
		return fail(fmt.Errorf("create build archive: %w", err))
	}
	if err := packDirectory(contextDir, tarFile, c.maxBytes); err != nil {
		_ = tarFile.Close()
		_ = os.Remove(tarFile.Name())
		return fail(err)
	}
	if _, err := tarFile.Seek(0, io.SeekStart); err != nil {
		_ = tarFile.Close()
		_ = os.Remove(tarFile.Name())
		return fail(fmt.Errorf("rewind build archive: %w", err))
	}

	head, err := repo.Head()
	if err != nil {
		_ = tarFile.Close()
		_ = os.Remove(tarFile.Name())
		return fail(fmt.Errorf("read repository HEAD: %w", err))
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		_ = tarFile.Close()
		_ = os.Remove(tarFile.Name())
		return fail(fmt.Errorf("read repository commit: %w", err))
	}
	return Snapshot{
		Source:         &cleanupFile{File: tarFile, paths: []string{tarFile.Name(), repoDir}},
		DockerfilePath: dockerfileRel,
		CommitSHA:      head.Hash().String(), CommitAuthor: truncateRunes(strings.TrimSpace(commit.Author.Name), 255),
		CommitMessage: truncateRunes(strings.TrimSpace(commit.Message), 4096), Branch: branch,
	}, nil
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func secureDirectory(root, rel string) (string, error) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("build context: %w", domain.ErrGitPathInvalid)
	}
	if !inside(root, resolved) {
		return "", domain.ErrGitPathInvalid
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("build context: %w", domain.ErrGitPathInvalid)
	}
	return resolved, nil
}

func secureDockerfile(contextDir, rel string) error {
	path := filepath.Join(contextDir, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || !inside(contextDir, path) {
		return domain.ErrDockerfileNotFound
	}
	return nil
}

func inside(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func packDirectory(root string, dst io.Writer, maxBytes int64) error {
	tw := tar.NewWriter(dst)
	defer tw.Close()
	var total int64
	return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return err
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.Mode().IsRegular() {
			total += info.Size()
			if maxBytes > 0 && total > maxBytes {
				return fmt.Errorf("repository build context exceeds the configured size limit")
			}
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

type cleanupFile struct {
	*os.File
	paths []string
}

func (f *cleanupFile) Close() error {
	err := f.File.Close()
	for _, path := range f.paths {
		_ = os.RemoveAll(path)
	}
	return err
}
