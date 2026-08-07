package tarball

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackContentsAndSkips(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Dockerfile"), "FROM scratch\n")
	writeFile(t, filepath.Join(root, "src", "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, ".env"), "SECRET=leak\n")
	writeFile(t, filepath.Join(root, ".env.example"), "SECRET=example\n")
	writeFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/x\n")
	writeFile(t, filepath.Join(root, "node_modules", "x.js"), "// junk\n")

	var buf bytes.Buffer
	if err := Pack(root, &buf); err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	tr := tar.NewReader(&buf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg {
			b, _ := io.ReadAll(tr)
			got[hdr.Name] = string(b)
		} else {
			got[hdr.Name] = ""
		}
	}

	if got["Dockerfile"] != "FROM scratch\n" {
		t.Errorf("Dockerfile missing/wrong: %q", got["Dockerfile"])
	}
	if got["src/main.go"] != "package main\n" {
		t.Errorf("src/main.go missing/wrong: %q", got["src/main.go"])
	}
	if _, ok := got[".env.example"]; !ok {
		t.Error(".env.example missing")
	}

	for _, path := range []string{".env", ".git/HEAD", "node_modules/x.js"} {
		if _, ok := got[path]; ok {
			t.Errorf("%s should be skipped", path)
		}
	}

	for name := range got {
		if bytes.ContainsRune([]byte(name), '\\') {
			t.Errorf("backslash in tar entry: %q", name)
		}
	}
}
