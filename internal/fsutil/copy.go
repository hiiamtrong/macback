package fsutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CopyFile copies a file from src to dst, preserving permissions.
func CopyFile(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	// Skip symlinks
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("skipping symlink: %s", src)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying data: %w", err)
	}

	return nil
}

// CopyDir recursively copies a directory from src to dst.
// It respects exclude patterns (glob-style) and skips matching files/dirs.
func CopyDir(src, dst string, excludes []string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source dir: %w", err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("creating destination dir: %w", err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Check excludes
		if shouldExclude(relPath, d.Name(), excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(dstPath, info.Mode().Perm())
		}

		// Skip symlinks
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		return CopyFile(path, dstPath)
	})
}

// shouldExclude checks if a path matches any exclude pattern.
func shouldExclude(relPath, name string, excludes []string) bool {
	for _, pattern := range excludes {
		// Check against basename
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		// Check against relative path components
		parts := strings.Split(relPath, string(filepath.Separator))
		for _, part := range parts {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}
	}
	return false
}

// SHA256File computes the SHA256 hash of a file.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file for hash: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("computing hash: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// FileExists checks if a file exists and is not a directory.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FileModeString formats a file mode as a string like "0644".
func FileModeString(mode fs.FileMode) string {
	return fmt.Sprintf("%04o", mode.Perm())
}

// ParseFileMode parses a mode string like "0644" to fs.FileMode.
func ParseFileMode(s string) (fs.FileMode, error) {
	var mode uint32
	_, err := fmt.Sscanf(s, "%o", &mode)
	if err != nil {
		return 0, fmt.Errorf("parsing file mode %q: %w", s, err)
	}
	return fs.FileMode(mode), nil
}
