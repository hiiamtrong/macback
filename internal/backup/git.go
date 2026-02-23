package backup

import (
	"context"
	"fmt"
	"os"

	"github.com/trongdev/macos-backup/internal/config"
	"github.com/trongdev/macos-backup/internal/crypto"
	"github.com/trongdev/macos-backup/internal/fsutil"
)

func init() {
	Register(&GitHandler{})
}

// GitHandler backs up git configuration files.
type GitHandler struct{}

func (h *GitHandler) Name() string { return "git" }

func (h *GitHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
	var entries []FileEntry

	for _, pattern := range cfg.Paths {
		expanded, err := fsutil.ExpandPath(pattern)
		if err != nil {
			continue
		}

		if !fsutil.FileExists(expanded) {
			continue
		}

		info, err := os.Stat(expanded)
		if err != nil {
			continue
		}

		entries = append(entries, FileEntry{
			SourcePath: expanded,
			RelPath:    info.Name(),
			Category:   "git",
			Mode:       info.Mode().Perm(),
			ModTime:    info.ModTime(),
			Size:       info.Size(),
		})
	}

	return entries, nil
}

func (h *GitHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
	result := &CategoryResult{
		CategoryName: "git",
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		me, err := BackupFileEntry(entry, dest, enc)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skipping %s: %v", entry.RelPath, err))
			continue
		}

		result.Entries = append(result.Entries, *me)
		result.FileCount++
		if me.Encrypted {
			result.EncryptedCount++
		}
	}

	return result, nil
}
