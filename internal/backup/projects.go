package backup

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiiamtrong/macback/internal/config"
	"github.com/hiiamtrong/macback/internal/crypto"
	"github.com/hiiamtrong/macback/internal/fsutil"
)

func init() {
	Register(&ProjectsHandler{})
}

// ProjectsHandler backs up developer project directories, skipping package
// install directories (node_modules, vendor, .venv, etc.) and large files.
type ProjectsHandler struct{}

func (h *ProjectsHandler) Name() string { return "projects" }

// Discover walks each scan_dir, finds project roots at project_depth, and
// collects all eligible files under those roots.
func (h *ProjectsHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
	var entries []FileEntry

	depth := cfg.ProjectDepth
	if depth <= 0 {
		depth = 1
	}

	for _, scanDir := range cfg.ScanDirs {
		expanded, err := fsutil.ExpandPath(scanDir)
		if err != nil || !fsutil.DirExists(expanded) {
			continue
		}

		projectRoots := collectProjectRoots(expanded, depth)
		for _, root := range projectRoots {
			fileEntries := discoverProjectFiles(expanded, root, cfg)
			entries = append(entries, fileEntries...)
		}
	}

	return entries, nil
}

// collectProjectRoots returns directories at exactly depth levels below base.
func collectProjectRoots(base string, depth int) []string {
	var roots []string

	_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == base {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(base, path)
		parts := strings.Split(rel, string(filepath.Separator))

		if len(parts) == depth {
			roots = append(roots, path)
			return filepath.SkipDir
		}
		if len(parts) > depth {
			return filepath.SkipDir
		}
		return nil
	})

	return roots
}

// discoverProjectFiles walks a single project directory, applying exclusions
// and the max file size filter.
func discoverProjectFiles(scanDir, projectDir string, cfg *config.CategoryConfig) []FileEntry {
	var entries []FileEntry
	maxBytes := projectMaxBytes(cfg)

	_ = filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relToProject, _ := filepath.Rel(projectDir, path)
		if path != projectDir && shouldExcludeEntry(relToProject, d.Name(), cfg.Exclude) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		entry := buildFileEntry(scanDir, path, d, maxBytes)
		if entry != nil {
			entries = append(entries, *entry)
		}
		return nil
	})

	return entries
}

// projectMaxBytes returns the max file size in bytes from config (0 = unlimited).
func projectMaxBytes(cfg *config.CategoryConfig) int64 {
	if cfg.MaxFileSizeMB > 0 {
		return int64(cfg.MaxFileSizeMB) * 1024 * 1024
	}
	return 0
}

// buildFileEntry constructs a FileEntry for a regular file, returning nil if
// the file should be skipped (symlink, above max size, or stat error).
func buildFileEntry(scanDir, path string, d fs.DirEntry, maxBytes int64) *FileEntry {
	info, err := d.Info()
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return nil
	}
	relPath, _ := filepath.Rel(scanDir, path)
	return &FileEntry{
		SourcePath: path,
		RelPath:    relPath,
		Category:   "projects",
		Mode:       info.Mode().Perm(),
		ModTime:    info.ModTime(),
		Size:       info.Size(),
	}
}

// Backup copies (or encrypts) each file entry to the destination directory.
func (h *ProjectsHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
	result := &CategoryResult{
		CategoryName: "projects",
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
			me.Path = filepath.Join("projects", entry.RelPath+filepath.Ext(encPath))
			me.Encrypted = true
		} else {
			if err := fsutil.CopyFile(entry.SourcePath, dstPath); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("copying %s: %v", entry.RelPath, err))
				continue
			}
			me.Path = filepath.Join("projects", entry.RelPath)
		}

		result.Entries = append(result.Entries, *me)
		result.FileCount++
		if me.Encrypted {
			result.EncryptedCount++
		}
	}

	return result, nil
}
