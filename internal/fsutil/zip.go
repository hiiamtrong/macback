package fsutil

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ZipDir creates a zip archive at destZip containing all files under srcDir.
// File paths inside the zip are relative to srcDir.
// The archive is written atomically (temp file + rename).
// Symlinks are skipped.
func ZipDir(srcDir, destZip string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destZip), 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// Write to a temp file first (atomic)
	tmp, err := os.CreateTemp(filepath.Dir(destZip), ".macback-zip-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Remove temp file on any error path
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	zw := zip.NewWriter(tmp)

	walkErr := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
			return nil // skip directories and symlinks
		}
		return addFileToZip(zw, srcDir, path, d)
	})

	if walkErr != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return fmt.Errorf("zipping %s: %w", srcDir, walkErr)
	}

	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("finalizing zip: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, destZip); err != nil {
		return fmt.Errorf("renaming zip to destination: %w", err)
	}

	success = true
	return nil
}

// addFileToZip adds a single file to the zip writer with a path relative to srcDir.
func addFileToZip(zw *zip.Writer, srcDir, path string, d fs.DirEntry) error {
	rel, err := filepath.Rel(srcDir, path)
	if err != nil {
		return err
	}
	// Use forward slashes inside zip (zip spec)
	rel = filepath.ToSlash(rel)

	info, err := d.Info()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = rel
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(w, f)
	return err
}

// UnzipToTemp extracts zipPath into a new temporary directory.
// Returns the temp dir path and a cleanup function that removes it.
// Caller must call cleanup() when done (typically deferred).
// Rejects zip entries containing ".." to prevent path traversal (zip-slip).
func UnzipToTemp(zipPath string) (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "macback-restore-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("opening zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		// Zip-slip guard: reject any path component that is ".."
		if slices.Contains(strings.Split(filepath.ToSlash(f.Name), "/"), "..") {
			cleanup()
			return "", nil, fmt.Errorf("zip entry %q contains unsafe path traversal", f.Name)
		}

		dest := filepath.Join(dir, filepath.FromSlash(f.Name))

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0755); err != nil {
				cleanup()
				return "", nil, fmt.Errorf("creating directory %s: %w", dest, err)
			}
			continue
		}

		if err := extractZipEntry(f, dest); err != nil {
			cleanup()
			return "", nil, err
		}
	}

	return dir, cleanup, nil
}

// extractZipEntry extracts a single zip file entry to dest.
func extractZipEntry(f *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("creating parent dir for %s: %w", dest, err)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening zip entry %s: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	mode := f.FileInfo().Mode().Perm()
	if mode == 0 {
		mode = 0644
	}

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", dest, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("extracting %s: %w", f.Name, err)
	}

	return nil
}
