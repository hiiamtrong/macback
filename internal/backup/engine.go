package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/trongdev/macos-backup/internal/config"
	"github.com/trongdev/macos-backup/internal/crypto"
	"github.com/trongdev/macos-backup/internal/fsutil"
)

// Engine orchestrates the backup process across all categories.
type Engine struct {
	cfg *config.Config
	enc crypto.Encryptor
}

// NewEngine creates a backup engine from config.
func NewEngine(cfg *config.Config, enc crypto.Encryptor) *Engine {
	return &Engine{cfg: cfg, enc: enc}
}

// Run executes the backup for the specified categories (or all enabled if empty).
func (e *Engine) Run(ctx context.Context, categories []string, dest string) (*Manifest, error) {
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
			fmt.Fprintf(os.Stderr, "Warning: no handler for category %q\n", catName)
			continue
		}

		fmt.Printf("Backing up %s...\n", catName)

		entries, err := handler.Discover(catCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s discovery failed: %v\n", catName, err)
			continue
		}

		// Mark secrets
		for i := range entries {
			if crypto.IsSecret(entries[i].RelPath, catCfg.SecretPatterns, e.cfg.Encryption.GlobalSecretPatterns) {
				entries[i].IsSecret = true
			}
		}

		catDest := filepath.Join(dest, catName)
		result, err := handler.Backup(ctx, entries, catDest, e.enc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s backup failed: %v\n", catName, err)
			continue
		}

		manifest.Categories[catName] = &ManifestCategory{
			BackedUp:       true,
			FileCount:      result.FileCount,
			EncryptedCount: result.EncryptedCount,
			Files:          result.Entries,
		}

		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "  Warning: %s\n", w)
		}

		fmt.Printf("  %d files backed up", result.FileCount)
		if result.EncryptedCount > 0 {
			fmt.Printf(" (%d encrypted)", result.EncryptedCount)
		}
		fmt.Println()
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
			fmt.Fprintf(os.Stderr, "Warning: %s discovery failed: %v\n", catName, err)
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
