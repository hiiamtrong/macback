package backup

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/trongdev/macos-backup/internal/config"
	"github.com/trongdev/macos-backup/internal/crypto"
	"github.com/trongdev/macos-backup/internal/fsutil"
)

func init() {
	Register(&AppSettingsHandler{})
}

// AppSettingsHandler backs up application preference plists.
type AppSettingsHandler struct{}

func (h *AppSettingsHandler) Name() string { return "appsettings" }

func (h *AppSettingsHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
	var entries []FileEntry

	for _, pattern := range cfg.Paths {
		expanded, err := fsutil.ExpandPath(pattern)
		if err != nil {
			continue
		}

		if strings.HasSuffix(pattern, "/") || fsutil.DirExists(expanded) {
			dirEntries, err := discoverPreferences(expanded, cfg.Exclude)
			if err != nil {
				continue
			}
			entries = append(entries, dirEntries...)
			continue
		}

		// Single file
		if !fsutil.FileExists(expanded) {
			continue
		}

		info, err := os.Stat(expanded)
		if err != nil {
			continue
		}

		relPath := filepath.Base(expanded)
		entries = append(entries, FileEntry{
			SourcePath: expanded,
			RelPath:    relPath,
			Category:   "appsettings",
			Mode:       info.Mode().Perm(),
			ModTime:    info.ModTime(),
			Size:       info.Size(),
		})
	}

	return entries, nil
}

// discoverPreferences walks a preferences directory, skipping excluded patterns.
func discoverPreferences(dir string, excludes []string) ([]FileEntry, error) {
	var entries []FileEntry

	if !fsutil.DirExists(dir) {
		return nil, nil
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			// Only walk the top-level directory
			if path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()

		// Only include .plist files
		if !strings.HasSuffix(name, ".plist") {
			return nil
		}

		// Check excludes
		if shouldExcludePref(name, excludes) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		entries = append(entries, FileEntry{
			SourcePath: path,
			RelPath:    name,
			Category:   "appsettings",
			Mode:       info.Mode().Perm(),
			ModTime:    info.ModTime(),
			Size:       info.Size(),
		})

		return nil
	})

	return entries, err
}

// shouldExcludePref checks if a filename matches any exclude pattern.
func shouldExcludePref(name string, excludes []string) bool {
	for _, pattern := range excludes {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

func (h *AppSettingsHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
	result := &CategoryResult{
		CategoryName: "appsettings",
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return nil, fmt.Errorf("creating appsettings backup dir: %w", err)
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		dstPath := filepath.Join(dest, entry.RelPath)

		hash, err := fsutil.SHA256File(entry.SourcePath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skipping %s: %v", entry.RelPath, err))
			continue
		}

		me := &ManifestEntry{
			Path:     filepath.Join("appsettings", entry.RelPath),
			Original: fsutil.ContractPath(entry.SourcePath),
			Size:     entry.Size,
			Mode:     fsutil.FileModeString(entry.Mode),
			ModTime:  entry.ModTime,
			SHA256:   hash,
		}

		if err := fsutil.CopyFile(entry.SourcePath, dstPath); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("copying %s: %v", entry.RelPath, err))
			continue
		}

		result.Entries = append(result.Entries, *me)
		result.FileCount++
	}

	return result, nil
}
