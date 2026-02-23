package restore

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/trongdev/macos-backup/internal/backup"
	"github.com/trongdev/macos-backup/internal/crypto"
	"github.com/trongdev/macos-backup/internal/fsutil"
	"github.com/trongdev/macos-backup/internal/logger"
)

// DiffStatus represents the state of a file comparison.
type DiffStatus string

const (
	DiffNew       DiffStatus = "new"       // File in backup but not on system
	DiffModified  DiffStatus = "modified"  // File differs between backup and system
	DiffIdentical DiffStatus = "identical" // File is the same
	DiffMissing   DiffStatus = "missing"   // File in manifest but missing from backup
)

// DiffEntry represents a single file comparison result.
type DiffEntry struct {
	Category   string
	BackupPath string
	SystemPath string
	Status     DiffStatus
	Encrypted  bool
}

// Result contains the outcome of a restore operation.
type Result struct {
	Restored int
	Skipped  int
	Errors   int
}

// Engine performs restore operations.
type Engine struct {
	dec crypto.Decryptor
	log *logger.Logger
}

// NewEngine creates a new restore engine.
func NewEngine(dec crypto.Decryptor, log *logger.Logger) *Engine {
	return &Engine{dec: dec, log: log}
}

// Diff compares the backup with the current system state.
func (e *Engine) Diff(ctx context.Context, manifest *backup.Manifest, backupDir string, categoryFilter []string) ([]DiffEntry, error) {
	var diffs []DiffEntry

	filterMap := buildFilterMap(categoryFilter)

	for catName, cat := range manifest.Categories {
		if !cat.BackedUp {
			continue
		}
		if filterMap != nil && !filterMap[catName] {
			continue
		}

		for _, f := range cat.Files {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			entry := DiffEntry{
				Category:   catName,
				BackupPath: f.Path,
				SystemPath: f.Original,
				Encrypted:  f.Encrypted,
			}

			// Check if backup file exists
			backupFilePath := filepath.Join(backupDir, f.Path)
			if !fsutil.FileExists(backupFilePath) {
				entry.Status = DiffMissing
				diffs = append(diffs, entry)
				continue
			}

			// Expand system path
			systemPath, err := fsutil.ExpandPath(f.Original)
			if err != nil {
				entry.Status = DiffNew
				diffs = append(diffs, entry)
				continue
			}

			// Check if system file exists
			if !fsutil.FileExists(systemPath) {
				entry.Status = DiffNew
				diffs = append(diffs, entry)
				continue
			}

			// Compare hashes (for non-encrypted files, compare directly)
			if !f.Encrypted && f.SHA256 != "" {
				systemHash, err := fsutil.SHA256File(systemPath)
				if err != nil {
					entry.Status = DiffModified
					diffs = append(diffs, entry)
					continue
				}

				if systemHash == f.SHA256 {
					entry.Status = DiffIdentical
				} else {
					entry.Status = DiffModified
				}
			} else {
				// For encrypted files, we can't easily compare without decrypting
				// Mark as modified (conservative)
				entry.Status = DiffModified
			}

			diffs = append(diffs, entry)
		}
	}

	return diffs, nil
}

// Run restores files from the backup to the system.
func (e *Engine) Run(ctx context.Context, manifest *backup.Manifest, backupDir string, categoryFilter []string, force bool) (*Result, error) {
	result := &Result{}

	filterMap := buildFilterMap(categoryFilter)

	for catName, cat := range manifest.Categories {
		if !cat.BackedUp {
			continue
		}
		if filterMap != nil && !filterMap[catName] {
			continue
		}

		e.log.Info("Restoring %s...", catName)

		for _, f := range cat.Files {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			default:
			}

			backupFilePath := filepath.Join(backupDir, f.Path)
			if !fsutil.FileExists(backupFilePath) {
				e.log.Warn("  backup file missing: %s", f.Path)
				result.Errors++
				continue
			}

			systemPath, err := fsutil.ExpandPath(f.Original)
			if err != nil {
				e.log.Warn("  cannot expand path %s: %v", f.Original, err)
				result.Errors++
				continue
			}

			// Check if system file exists and handle conflicts
			if fsutil.FileExists(systemPath) && !force {
				// Check if identical
				if !f.Encrypted && f.SHA256 != "" {
					systemHash, _ := fsutil.SHA256File(systemPath)
					if systemHash == f.SHA256 {
						e.log.Info("  %s [skipped - identical]", fsutil.ContractPath(systemPath))
						result.Skipped++
						continue
					}
				}

				// Ask user
				e.log.Infof("  %s already exists. Overwrite? [y/N/skip] ", fsutil.ContractPath(systemPath))
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))

				if answer != "y" && answer != "yes" {
					e.log.Info("  %s [skipped]", fsutil.ContractPath(systemPath))
					result.Skipped++
					continue
				}
			}

			// Ensure destination directory exists
			if err := os.MkdirAll(filepath.Dir(systemPath), 0755); err != nil {
				e.log.Error("  creating directory for %s: %v", systemPath, err)
				result.Errors++
				continue
			}

			// Restore the file
			if f.Encrypted {
				if err := e.dec.DecryptFile(backupFilePath, systemPath); err != nil {
					e.log.Error("  decrypting %s: %v", f.Path, err)
					result.Errors++
					continue
				}
			} else {
				if err := fsutil.CopyFile(backupFilePath, systemPath); err != nil {
					e.log.Error("  restoring %s: %v", f.Path, err)
					result.Errors++
					continue
				}
			}

			// Restore file permissions
			if f.Mode != "" {
				mode, err := fsutil.ParseFileMode(f.Mode)
				if err == nil {
					os.Chmod(systemPath, mode)
				}
			}

			status := "restored"
			if f.Encrypted {
				status = "decrypted and restored"
			}
			e.log.Info("  %s [%s]", fsutil.ContractPath(systemPath), status)
			result.Restored++
		}
	}

	return result, nil
}

// PrintDiffs prints the diff entries in a readable format.
func PrintDiffs(diffs []DiffEntry) {
	if len(diffs) == 0 {
		fmt.Println("No differences found. Backup matches current system.")
		return
	}

	currentCat := ""
	for _, d := range diffs {
		if d.Category != currentCat {
			currentCat = d.Category
			fmt.Printf("\n%s:\n", strings.ToUpper(currentCat))
		}

		var prefix string
		switch d.Status {
		case DiffNew:
			prefix = "[NEW]      "
		case DiffModified:
			prefix = "[MODIFIED] "
		case DiffIdentical:
			prefix = "[IDENTICAL]"
		case DiffMissing:
			prefix = "[MISSING]  "
		}

		encrypted := ""
		if d.Encrypted {
			encrypted = " (encrypted)"
		}

		fmt.Printf("  %s %s%s\n", prefix, d.SystemPath, encrypted)
	}
	fmt.Println()
}

// buildFilterMap creates a category filter map from a slice.
func buildFilterMap(categories []string) map[string]bool {
	if len(categories) == 0 {
		return nil
	}
	m := make(map[string]bool)
	for _, c := range categories {
		m[strings.TrimSpace(c)] = true
	}
	return m
}
