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
	Register(&DotfilesHandler{})
}

// DotfilesHandler backs up dotfiles and custom scripts.
type DotfilesHandler struct{}

func (h *DotfilesHandler) Name() string { return "dotfiles" }

func (h *DotfilesHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
	var entries []FileEntry

	for _, pattern := range cfg.Paths {
		expanded, err := fsutil.ExpandPath(pattern)
		if err != nil {
			continue
		}

		// Check if it's a directory path (ends with /)
		if strings.HasSuffix(pattern, "/") || fsutil.DirExists(expanded) {
			dirEntries, err := discoverDir(expanded, expanded, cfg.Exclude)
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

		// Get relative path from home
		home, _ := os.UserHomeDir()
		relPath := expanded
		if strings.HasPrefix(expanded, home) {
			relPath, _ = filepath.Rel(home, expanded)
		}

		entries = append(entries, FileEntry{
			SourcePath: expanded,
			RelPath:    relPath,
			Category:   "dotfiles",
			Mode:       info.Mode().Perm(),
			ModTime:    info.ModTime(),
			Size:       info.Size(),
		})
	}

	return entries, nil
}

func discoverDir(root, dir string, excludes []string) ([]FileEntry, error) {
	var entries []FileEntry

	home, _ := os.UserHomeDir()

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Check excludes
		relToRoot, _ := filepath.Rel(root, path)
		if shouldExcludeEntry(relToRoot, d.Name(), excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Skip symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// Get relative path from home for storage
		relPath := path
		if strings.HasPrefix(path, home) {
			relPath, _ = filepath.Rel(home, path)
		}

		entries = append(entries, FileEntry{
			SourcePath: path,
			RelPath:    relPath,
			Category:   "dotfiles",
			Mode:       info.Mode().Perm(),
			ModTime:    info.ModTime(),
			Size:       info.Size(),
		})

		return nil
	})

	return entries, err
}

func shouldExcludeEntry(relPath, name string, excludes []string) bool {
	for _, pattern := range excludes {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		parts := strings.Split(relPath, string(filepath.Separator))
		for _, part := range parts {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}
	}
	return false
}

func (h *DotfilesHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
	result := &CategoryResult{
		CategoryName: "dotfiles",
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
			Original:  fsutil.ContractPath(entry.SourcePath),
			Size:      entry.Size,
			Mode:      fsutil.FileModeString(entry.Mode),
			ModTime:   entry.ModTime,
			SHA256:    hash,
			Encrypted: false,
		}

		if entry.IsSecret {
			encPath, err := enc.EncryptFile(entry.SourcePath, dstPath)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("encrypting %s: %v", entry.RelPath, err))
				continue
			}
			me.Path = filepath.Join("dotfiles", entry.RelPath+filepath.Ext(encPath))
			me.Encrypted = true
		} else {
			if err := fsutil.CopyFile(entry.SourcePath, dstPath); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("copying %s: %v", entry.RelPath, err))
				continue
			}
			me.Path = filepath.Join("dotfiles", entry.RelPath)
		}

		result.Entries = append(result.Entries, *me)
		result.FileCount++
		if me.Encrypted {
			result.EncryptedCount++
		}
	}

	return result, nil
}
