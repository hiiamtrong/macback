package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hiiamtrong/macback/internal/config"
	"github.com/hiiamtrong/macback/internal/crypto"
	"github.com/hiiamtrong/macback/internal/fsutil"
)

func init() {
	Register(&SSHHandler{})
}

// SSHHandler backs up SSH configuration and keys.
type SSHHandler struct{}

func (h *SSHHandler) Name() string { return "ssh" }

func (h *SSHHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
	var entries []FileEntry

	for _, pattern := range cfg.Paths {
		matches, err := fsutil.ExpandGlob(pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			info, err := os.Lstat(match)
			if err != nil {
				continue
			}
			if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				continue
			}

			// Check excludes
			excluded := false
			for _, excl := range cfg.Exclude {
				if matched, _ := filepath.Match(excl, info.Name()); matched {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}

			entries = append(entries, FileEntry{
				SourcePath: match,
				RelPath:    info.Name(),
				Category:   "ssh",
				Mode:       info.Mode().Perm(),
				ModTime:    info.ModTime(),
				Size:       info.Size(),
			})
		}
	}

	return entries, nil
}

func (h *SSHHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
	result := &CategoryResult{
		CategoryName: "ssh",
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
