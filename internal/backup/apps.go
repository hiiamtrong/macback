package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/trongdev/macos-backup/internal/config"
	"github.com/trongdev/macos-backup/internal/crypto"
	"github.com/trongdev/macos-backup/internal/fsutil"
	"gopkg.in/yaml.v3"
)

func init() {
	Register(&AppsHandler{})
}

// AppsHandler catalogs installed applications.
type AppsHandler struct{}

func (h *AppsHandler) Name() string { return "apps" }

// AppsCatalog is the YAML structure for the apps catalog.
type AppsCatalog struct {
	ScannedAt   time.Time      `yaml:"scanned_at"`
	Directories []ScannedApps  `yaml:"directories"`
	Summary     AppsSummary    `yaml:"summary"`
}

type ScannedApps struct {
	Path string     `yaml:"path"`
	Apps []AppEntry `yaml:"apps"`
}

type AppEntry struct {
	Name   string `yaml:"name"`
	Size   int64  `yaml:"size_bytes"`
	Source string `yaml:"source"` // "app-store", "homebrew-cask", "manual"
}

type AppsSummary struct {
	Total        int `yaml:"total"`
	AppStore     int `yaml:"app_store"`
	HomebrewCask int `yaml:"homebrew_cask"`
	Manual       int `yaml:"manual"`
}

func (h *AppsHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
	return []FileEntry{{Category: "apps", RelPath: "apps-catalog.yaml"}}, nil
}

func (h *AppsHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
	result := &CategoryResult{
		CategoryName: "apps",
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return nil, fmt.Errorf("creating apps backup dir: %w", err)
	}

	// Get homebrew casks for source detection
	brewCasks := getBrewCasks(ctx)

	scanDirs := []string{"/Applications", "~/Applications"}

	catalog := AppsCatalog{
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

		scanned := ScannedApps{Path: expanded}

		dirEntries, err := os.ReadDir(expanded)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("cannot read %s: %v", dir, err))
			continue
		}

		for _, de := range dirEntries {
			if !strings.HasSuffix(de.Name(), ".app") {
				continue
			}

			appName := strings.TrimSuffix(de.Name(), ".app")
			appPath := filepath.Join(expanded, de.Name())

			size := getAppSize(appPath)
			source := detectAppSource(appPath, appName, brewCasks)

			scanned.Apps = append(scanned.Apps, AppEntry{
				Name:   appName,
				Size:   size,
				Source: source,
			})

			// Update summary
			switch source {
			case "app-store":
				catalog.Summary.AppStore++
			case "homebrew-cask":
				catalog.Summary.HomebrewCask++
			default:
				catalog.Summary.Manual++
			}
			catalog.Summary.Total++
		}

		if len(scanned.Apps) > 0 {
			catalog.Directories = append(catalog.Directories, scanned)
		}
	}

	// Write catalog
	catalogPath := filepath.Join(dest, "apps-catalog.yaml")
	data, err := yaml.Marshal(catalog)
	if err != nil {
		return nil, fmt.Errorf("marshaling apps catalog: %w", err)
	}

	if err := os.WriteFile(catalogPath, data, 0644); err != nil {
		return nil, fmt.Errorf("writing apps catalog: %w", err)
	}

	result.Entries = append(result.Entries, ManifestEntry{
		Path:     "apps/apps-catalog.yaml",
		Original: "apps-catalog.yaml",
		Size:     int64(len(data)),
	})
	result.FileCount++

	return result, nil
}

// getBrewCasks returns a set of app names installed via Homebrew Cask.
func getBrewCasks(ctx context.Context) map[string]bool {
	casks := make(map[string]bool)

	output, err := exec.CommandContext(ctx, "brew", "list", "--cask", "-1").Output()
	if err != nil {
		return casks
	}

	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			casks[strings.ToLower(line)] = true
		}
	}

	return casks
}

// detectAppSource determines how an app was installed.
func detectAppSource(appPath, appName string, brewCasks map[string]bool) string {
	// Check for Mac App Store receipt
	receiptPath := filepath.Join(appPath, "Contents", "_MASReceipt", "receipt")
	if fsutil.FileExists(receiptPath) {
		return "app-store"
	}

	// Check if app name matches a Homebrew cask
	normalizedName := strings.ToLower(strings.ReplaceAll(appName, " ", "-"))
	if brewCasks[normalizedName] {
		return "homebrew-cask"
	}

	// Also try without hyphens
	noHyphens := strings.ToLower(strings.ReplaceAll(appName, " ", ""))
	if brewCasks[noHyphens] {
		return "homebrew-cask"
	}

	return "manual"
}

// getAppSize returns the total size of an app bundle.
func getAppSize(appPath string) int64 {
	var total int64
	_ = filepath.WalkDir(appPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}
