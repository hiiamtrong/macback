package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/trongdev/macos-backup/internal/config"
	"github.com/trongdev/macos-backup/internal/crypto"
	"github.com/trongdev/macos-backup/internal/fsutil"
	"gopkg.in/yaml.v3"
)

func init() {
	Register(&PathBinHandler{})
}

// PathBinHandler catalogs custom binaries in PATH directories.
type PathBinHandler struct{}

func (h *PathBinHandler) Name() string { return "pathbin" }

// BinaryCatalog is the YAML structure for the binary catalog.
type BinaryCatalog struct {
	ScannedAt   time.Time    `yaml:"scanned_at"`
	Directories []ScannedDir `yaml:"directories"`
}

type ScannedDir struct {
	Path     string        `yaml:"path"`
	Binaries []BinaryEntry `yaml:"binaries"`
}

type BinaryEntry struct {
	Name          string `yaml:"name"`
	Size          int64  `yaml:"size"`
	Mode          string `yaml:"mode"`
	IsSymlink     bool   `yaml:"is_symlink"`
	SymlinkTarget string `yaml:"symlink_target,omitempty"`
	Source        string `yaml:"source"`
}

func (h *PathBinHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
	return []FileEntry{{Category: "pathbin", RelPath: "catalog.yaml"}}, nil
}

func (h *PathBinHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
	result := &CategoryResult{
		CategoryName: "pathbin",
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return nil, fmt.Errorf("creating pathbin backup dir: %w", err)
	}

	// Get the config from the engine (passed through entries)
	// We need to scan directories - get scan dirs from a default if not configured
	scanDirs := []string{"/usr/local/bin", "~/bin", "~/go/bin", "~/.local/bin"}
	excludePrefixes := []string{"/usr/bin", "/bin", "/sbin"}

	catalog := BinaryCatalog{
		ScannedAt: time.Now(),
	}

	for _, dir := range scanDirs {
		expanded, err := fsutil.ExpandPath(dir)
		if err != nil {
			continue
		}

		if !fsutil.DirExists(expanded) {
			continue
		}

		scanned := ScannedDir{Path: expanded}

		dirEntries, err := os.ReadDir(expanded)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("cannot read %s: %v", dir, err))
			continue
		}

		for _, de := range dirEntries {
			if de.IsDir() {
				continue
			}

			info, err := de.Info()
			if err != nil {
				continue
			}

			// Check if executable
			if info.Mode().Perm()&0111 == 0 {
				continue
			}

			entry := BinaryEntry{
				Name: de.Name(),
				Size: info.Size(),
				Mode: fsutil.FileModeString(info.Mode()),
			}

			// Check if symlink
			fullPath := filepath.Join(expanded, de.Name())
			if linfo, err := os.Lstat(fullPath); err == nil && linfo.Mode()&os.ModeSymlink != 0 {
				entry.IsSymlink = true
				if target, err := os.Readlink(fullPath); err == nil {
					entry.SymlinkTarget = target
				}
			}

			// Detect source
			entry.Source = detectOrigin(fullPath, excludePrefixes)

			scanned.Binaries = append(scanned.Binaries, entry)
		}

		if len(scanned.Binaries) > 0 {
			catalog.Directories = append(catalog.Directories, scanned)
		}
	}

	// Write catalog
	catalogPath := filepath.Join(dest, "catalog.yaml")
	data, err := yaml.Marshal(catalog)
	if err != nil {
		return nil, fmt.Errorf("marshaling catalog: %w", err)
	}

	if err := os.WriteFile(catalogPath, data, 0644); err != nil {
		return nil, fmt.Errorf("writing catalog: %w", err)
	}

	result.Entries = append(result.Entries, ManifestEntry{
		Path:     "pathbin/catalog.yaml",
		Original: "catalog.yaml",
		Size:     int64(len(data)),
	})
	result.FileCount++

	return result, nil
}

func detectOrigin(binPath string, excludePrefixes []string) string {
	resolved, err := filepath.EvalSymlinks(binPath)
	if err != nil {
		resolved = binPath
	}

	// Check excludes
	for _, prefix := range excludePrefixes {
		if strings.HasPrefix(resolved, prefix) {
			return "system"
		}
	}

	switch {
	case strings.Contains(resolved, "/Cellar/") || strings.Contains(resolved, "/homebrew/"):
		return "homebrew"
	case strings.Contains(resolved, "/go/bin/"):
		return "go"
	case strings.Contains(resolved, "/cargo/bin/"):
		return "cargo"
	case strings.Contains(resolved, "/.npm/") || strings.Contains(resolved, "/node_modules/"):
		return "npm"
	default:
		return "manual"
	}
}
