package backup

import (
	"context"
	"io/fs"
	"time"

	"github.com/hiiamtrong/macback/internal/config"
	"github.com/hiiamtrong/macback/internal/crypto"
)

// Category represents a backup category handler.
type Category interface {
	Name() string
	Discover(cfg *config.CategoryConfig) ([]FileEntry, error)
	Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error)
}

// FileEntry represents a file discovered for backup.
type FileEntry struct {
	SourcePath string
	RelPath    string
	Category   string
	IsSecret   bool
	IsDir      bool
	Mode       fs.FileMode
	ModTime    time.Time
	Size       int64
}

// CategoryResult holds the result of a backup operation for one category.
type CategoryResult struct {
	CategoryName   string
	FileCount      int
	EncryptedCount int
	SkippedCount   int
	Entries        []ManifestEntry
	Warnings       []string
}

// registry holds all registered category handlers.
var registry = make(map[string]Category)

// Register adds a category handler to the registry.
func Register(cat Category) {
	registry[cat.Name()] = cat
}

// GetCategory returns a registered category handler by name.
func GetCategory(name string) (Category, bool) {
	cat, ok := registry[name]
	return cat, ok
}

// AllCategories returns all registered category names.
func AllCategories() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
