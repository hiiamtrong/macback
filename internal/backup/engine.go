package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hiiamtrong/macback/internal/config"
	"github.com/hiiamtrong/macback/internal/crypto"
	"github.com/hiiamtrong/macback/internal/fsutil"
	"github.com/hiiamtrong/macback/internal/logger"
)

// Engine orchestrates the backup process across all categories.
type Engine struct {
	cfg *config.Config
	enc crypto.Encryptor
	log *logger.Logger
}

// NewEngine creates a backup engine from config.
func NewEngine(cfg *config.Config, enc crypto.Encryptor, log *logger.Logger) *Engine {
	return &Engine{cfg: cfg, enc: enc, log: log}
}

// Run executes the backup for the specified categories (or all enabled if empty).
func (e *Engine) Run(ctx context.Context, categories []string, dest string) (*Manifest, error) {
	// Load previous manifest for incremental backup (before rotation)
	var prevManifest *Manifest
	if prev, err := ReadManifest(dest); err == nil {
		prevManifest = prev
	}

	// Rotate existing backup if present, track rotated path for incremental checks
	var rotatedDir string
	if prevManifest != nil {
		rotatedDir = e.rotateBackup(dest, prevManifest)
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return nil, fmt.Errorf("creating backup destination: %w", err)
	}

	manifest := NewManifest("dev")

	cats := e.resolveCategories(categories)

	for _, catName := range cats {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		catCfg, ok := e.cfg.Categories[catName]
		if !ok || !catCfg.Enabled {
			continue
		}

		handler, ok := GetCategory(catName)
		if !ok {
			e.log.Warn("no handler for category %q", catName)
			continue
		}

		e.log.Info("Backing up %s...", catName)

		entries, err := handler.Discover(catCfg)
		if err != nil {
			e.log.Warn("%s discovery failed: %v", catName, err)
			continue
		}

		// Mark secrets
		for i := range entries {
			if crypto.IsSecret(entries[i].RelPath, catCfg.SecretPatterns, e.cfg.Encryption.GlobalSecretPatterns) {
				entries[i].IsSecret = true
			}
		}

		catDest := filepath.Join(dest, catName)

		// Incremental: skip unchanged files
		prevEntries := make(map[string]*ManifestEntry)
		if prevManifest != nil {
			if prevCat, ok := prevManifest.Categories[catName]; ok {
				for i := range prevCat.Files {
					prevEntries[prevCat.Files[i].Original] = &prevCat.Files[i]
				}
			}
		}

		var changedEntries []FileEntry
		var skippedEntries []ManifestEntry
		for _, entry := range entries {
			if entry.SourcePath == "" {
				changedEntries = append(changedEntries, entry)
				continue
			}

			hash, err := fsutil.SHA256File(entry.SourcePath)
			if err != nil {
				changedEntries = append(changedEntries, entry)
				continue
			}

			contracted := fsutil.ContractPath(entry.SourcePath)
			if prev, ok := prevEntries[contracted]; ok && prev.SHA256 == hash {
				// Try to reuse file from rotated dir (copy to new dest so backup is self-contained)
				if copyFromPrevious(catDest, rotatedDir, catName, entry) {
					skippedEntries = append(skippedEntries, *prev)
					continue
				}
			}

			changedEntries = append(changedEntries, entry)
		}

		result, err := handler.Backup(ctx, changedEntries, catDest, e.enc)
		if err != nil {
			e.log.Warn("%s backup failed: %v", catName, err)
			continue
		}

		// Add skipped entries back to result
		result.Entries = append(skippedEntries, result.Entries...)
		result.SkippedCount = len(skippedEntries)
		result.FileCount += len(skippedEntries)
		for _, se := range skippedEntries {
			if se.Encrypted {
				result.EncryptedCount++
			}
		}

		manifest.Categories[catName] = &ManifestCategory{
			BackedUp:       true,
			FileCount:      result.FileCount,
			EncryptedCount: result.EncryptedCount,
			Files:          result.Entries,
		}

		for _, w := range result.Warnings {
			e.log.Warn("  %s", w)
		}

		if result.SkippedCount > 0 {
			e.log.Info("  %d files backed up (%d unchanged)", result.FileCount-result.SkippedCount, result.SkippedCount)
		} else if result.EncryptedCount > 0 {
			e.log.Info("  %d files backed up (%d encrypted)", result.FileCount, result.EncryptedCount)
		} else {
			e.log.Info("  %d files backed up", result.FileCount)
		}
	}

	if err := WriteManifest(manifest, dest); err != nil {
		return nil, err
	}

	return manifest, nil
}

// DryRun performs discovery only, returning what would be backed up.
func (e *Engine) DryRun(ctx context.Context, categories []string) ([]FileEntry, error) {
	var allEntries []FileEntry

	cats := e.resolveCategories(categories)

	for _, catName := range cats {
		catCfg, ok := e.cfg.Categories[catName]
		if !ok || !catCfg.Enabled {
			continue
		}

		handler, ok := GetCategory(catName)
		if !ok {
			continue
		}

		entries, err := handler.Discover(catCfg)
		if err != nil {
			e.log.Warn("%s discovery failed: %v", catName, err)
			continue
		}

		for i := range entries {
			entries[i].Category = catName
			if crypto.IsSecret(entries[i].RelPath, catCfg.SecretPatterns, e.cfg.Encryption.GlobalSecretPatterns) {
				entries[i].IsSecret = true
			}
		}

		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

// resolveCategories returns the list of categories to process.
func (e *Engine) resolveCategories(filter []string) []string {
	if len(filter) > 0 {
		return filter
	}

	cats := e.cfg.EnabledCategories()
	sort.Strings(cats)
	return cats
}

// rotateBackup renames the existing backup directory using the previous manifest's timestamp.
// Returns the rotated directory path, or empty string if rotation didn't happen.
func (e *Engine) rotateBackup(dest string, prevManifest *Manifest) string {
	timestamp := prevManifest.CreatedAt.Format("2006-01-02-150405")
	rotatedDir := dest + "." + timestamp

	if fsutil.DirExists(rotatedDir) {
		return "" // Already rotated
	}

	e.log.Verbose("Rotating previous backup to %s", fsutil.ContractPath(rotatedDir))
	if err := os.Rename(dest, rotatedDir); err != nil {
		e.log.Warn("backup rotation failed: %v", err)
		return ""
	}

	// Clean up old rotated backups
	if err := e.cleanOldBackups(dest); err != nil {
		e.log.Warn("cleaning old backups: %v", err)
	}
	return rotatedDir
}

// copyFromPrevious copies an unchanged file from the rotated backup dir to the new catDest.
// Returns true if the file was successfully copied (or already exists), false otherwise.
func copyFromPrevious(catDest, rotatedDir, catName string, entry FileEntry) bool {
	relPath := entry.RelPath
	if entry.IsSecret {
		relPath = relPath + ".age"
	}

	destPath := filepath.Join(catDest, relPath)

	// Already exists in new dest
	if fsutil.FileExists(destPath) {
		return true
	}

	// Try to copy from rotated dir
	if rotatedDir != "" {
		srcPath := filepath.Join(rotatedDir, catName, relPath)
		if fsutil.FileExists(srcPath) {
			if err := fsutil.CopyFile(srcPath, destPath); err == nil {
				return true
			}
		}
	}

	return false
}

// cleanOldBackups removes rotated backups beyond MaxBackups limit.
func (e *Engine) cleanOldBackups(dest string) error {
	pattern := dest + ".*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}

	if len(matches) <= e.cfg.MaxBackups {
		return nil
	}

	// Sort by name (timestamps sort lexicographically)
	sort.Strings(matches)

	// Remove oldest, keep MaxBackups most recent
	toRemove := matches[:len(matches)-e.cfg.MaxBackups]
	for _, old := range toRemove {
		e.log.Verbose("Removing old backup: %s", fsutil.ContractPath(old))
		if err := os.RemoveAll(old); err != nil {
			e.log.Warn("failed to remove old backup %s: %v", old, err)
		}
	}

	return nil
}

// BackupFileEntry backs up a single file entry (used by category handlers).
func BackupFileEntry(entry FileEntry, dest string, enc crypto.Encryptor) (*ManifestEntry, error) {
	dstPath := filepath.Join(dest, entry.RelPath)

	hash, err := fsutil.SHA256File(entry.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("hashing %s: %w", entry.SourcePath, err)
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
			return nil, fmt.Errorf("encrypting %s: %w", entry.SourcePath, err)
		}
		me.Path = filepath.Join(filepath.Base(filepath.Dir(encPath)), filepath.Base(encPath))
		me.Encrypted = true
	} else {
		if err := fsutil.CopyFile(entry.SourcePath, dstPath); err != nil {
			return nil, fmt.Errorf("copying %s: %w", entry.SourcePath, err)
		}
		me.Path = filepath.Join(filepath.Base(filepath.Dir(dstPath)), filepath.Base(dstPath))
	}

	return me, nil
}
