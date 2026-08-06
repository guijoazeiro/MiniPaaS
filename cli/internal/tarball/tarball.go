package tarball

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Pack(root string, w io.Writer) error {
	root = filepath.Clean(root)
	tw := tar.NewWriter(w)
	defer tw.Close()

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if shouldSkip(rel, info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("header %s: %w", rel, err)
		}
		hdr.Name = filepath.ToSlash(rel)
		if info.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write header %s: %w", rel, err)
		}
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", rel, err)
		}
		_, cpErr := io.Copy(tw, f)
		_ = f.Close()
		if cpErr != nil {
			return fmt.Errorf("copy %s: %w", rel, cpErr)
		}
		return nil
	})
}

var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".next":        true,
	"vendor":       true,
	"dist":         true,
	"bin":          true,
}

func shouldSkip(rel string, info os.FileInfo) bool {
	base := filepath.Base(rel)
	if info.IsDir() && skipDirs[base] {
		return true
	}
	if !info.IsDir() && strings.HasPrefix(base, ".env") && base != ".env.example" {
		return true
	}
	return false
}
