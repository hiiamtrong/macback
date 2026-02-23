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
	Register(&ShellHandler{})
}

// ShellHandler backs up shell configuration files.
type ShellHandler struct{}

func (h *ShellHandler) Name() string { return "shell" }

func (h *ShellHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
	var entries []FileEntry

	for _, pattern := range cfg.Paths {
		matches, err := fsutil.ExpandGlob(pattern)
		if err != nil {
			continue
		}

		if len(matches) == 0 {
			// Try as a direct path
			expanded, err := fsutil.ExpandPath(pattern)
			if err != nil {
				continue
			}
			if !fsutil.FileExists(expanded) {
				continue
			}
			matches = []string{expanded}
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			if info.IsDir() {
				continue
			}

			entries = append(entries, FileEntry{
				SourcePath: match,
				RelPath:    info.Name(),
				Category:   "shell",
				Mode:       info.Mode().Perm(),
				ModTime:    info.ModTime(),
				Size:       info.Size(),
			})
		}
	}

	return entries, nil
}

func (h *ShellHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
	result := &CategoryResult{
		CategoryName: "shell",
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
