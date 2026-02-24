package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hiiamtrong/macback/internal/config"
	"github.com/hiiamtrong/macback/internal/crypto"
)

func init() {
	Register(&HomebrewHandler{})
}

// HomebrewHandler exports Homebrew package lists.
type HomebrewHandler struct{}

func (h *HomebrewHandler) Name() string { return "homebrew" }

func (h *HomebrewHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
	// Homebrew doesn't discover files - it runs commands
	// Return a single placeholder entry
	if _, err := exec.LookPath("brew"); err != nil {
		return nil, fmt.Errorf("homebrew is not installed")
	}
	return []FileEntry{{Category: "homebrew", RelPath: "Brewfile"}}, nil
}

func (h *HomebrewHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
	result := &CategoryResult{
		CategoryName: "homebrew",
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return nil, fmt.Errorf("creating homebrew backup dir: %w", err)
	}

	// Dump Brewfile
	brewfilePath := filepath.Join(dest, "Brewfile")
	cmd := exec.CommandContext(ctx, "brew", "bundle", "dump", "--file="+brewfilePath, "--force")
	if output, err := cmd.CombinedOutput(); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("brew bundle dump failed: %v: %s", err, output))
	} else {
		info, _ := os.Stat(brewfilePath)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		result.Entries = append(result.Entries, ManifestEntry{
			Path:     "homebrew/Brewfile",
			Original: "Brewfile",
			Size:     size,
		})
		result.FileCount++
	}

	// brew list --formula
	formulaPath := filepath.Join(dest, "brew-list.txt")
	if output, err := exec.CommandContext(ctx, "brew", "list", "--formula", "-1").Output(); err == nil {
		if err := os.WriteFile(formulaPath, output, 0644); err == nil {
			result.Entries = append(result.Entries, ManifestEntry{
				Path:     "homebrew/brew-list.txt",
				Original: "brew-list.txt",
				Size:     int64(len(output)),
			})
			result.FileCount++
		}
	}

	// brew list --cask
	caskPath := filepath.Join(dest, "brew-cask-list.txt")
	if output, err := exec.CommandContext(ctx, "brew", "list", "--cask", "-1").Output(); err == nil {
		if err := os.WriteFile(caskPath, output, 0644); err == nil {
			result.Entries = append(result.Entries, ManifestEntry{
				Path:     "homebrew/brew-cask-list.txt",
				Original: "brew-cask-list.txt",
				Size:     int64(len(output)),
			})
			result.FileCount++
		}
	}

	// brew tap
	tapPath := filepath.Join(dest, "brew-taps.txt")
	if output, err := exec.CommandContext(ctx, "brew", "tap").Output(); err == nil {
		if err := os.WriteFile(tapPath, output, 0644); err == nil {
			result.Entries = append(result.Entries, ManifestEntry{
				Path:     "homebrew/brew-taps.txt",
				Original: "brew-taps.txt",
				Size:     int64(len(output)),
			})
			result.FileCount++
		}
	}

	return result, nil
}
