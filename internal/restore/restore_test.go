package restore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/trongdev/macos-backup/internal/backup"
	"github.com/trongdev/macos-backup/internal/crypto"
	"github.com/trongdev/macos-backup/internal/fsutil"
	"github.com/trongdev/macos-backup/internal/logger"
)

func TestBuildFilterMap(t *testing.T) {
	tests := []struct {
		name       string
		categories []string
		wantNil    bool
		wantKeys   []string
	}{
		{
			name:       "empty slice returns nil",
			categories: []string{},
			wantNil:    true,
		},
		{
			name:       "nil slice returns nil",
			categories: nil,
			wantNil:    true,
		},
		{
			name:       "single category",
			categories: []string{"ssh"},
			wantNil:    false,
			wantKeys:   []string{"ssh"},
		},
		{
			name:       "multiple categories",
			categories: []string{"ssh", "git"},
			wantNil:    false,
			wantKeys:   []string{"ssh", "git"},
		},
		{
			name:       "trims whitespace",
			categories: []string{" ssh ", " git "},
			wantNil:    false,
			wantKeys:   []string{"ssh", "git"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFilterMap(tt.categories)
			if tt.wantNil {
				if got != nil {
					t.Errorf("buildFilterMap(%v) = %v, want nil", tt.categories, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("buildFilterMap(%v) = nil, want non-nil map", tt.categories)
			}
			for _, key := range tt.wantKeys {
				if !got[key] {
					t.Errorf("buildFilterMap result missing key %q", key)
				}
			}
			if len(got) != len(tt.wantKeys) {
				t.Errorf("buildFilterMap result has %d keys, want %d", len(got), len(tt.wantKeys))
			}
		})
	}
}

func TestDiffIdentical(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	systemDir := filepath.Join(dir, "system")
	if err := os.MkdirAll(filepath.Join(backupDir, "ssh"), 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(systemDir, 0755); err != nil { t.Fatal(err) }

	content := []byte("identical content here")

	// Create backup file
	backupFilePath := filepath.Join(backupDir, "ssh", "config")
	if err := os.WriteFile(backupFilePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create system file with identical content
	systemFilePath := filepath.Join(systemDir, "config")
	if err := os.WriteFile(systemFilePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Compute hash of the content
	hash, err := fsutil.SHA256File(systemFilePath)
	if err != nil {
		t.Fatal(err)
	}

	manifest := &backup.Manifest{
		Categories: map[string]*backup.ManifestCategory{
			"ssh": {
				BackedUp:  true,
				FileCount: 1,
				Files: []backup.ManifestEntry{
					{
						Path:      "ssh/config",
						Original:  systemFilePath,
						SHA256:    hash,
						Encrypted: false,
					},
				},
			},
		},
	}

	engine := NewEngine(&crypto.NullDecryptor{}, logger.New(false))
	diffs, err := engine.Diff(context.Background(), manifest, backupDir, nil)
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1", len(diffs))
	}
	if diffs[0].Status != DiffIdentical {
		t.Errorf("status = %q, want %q", diffs[0].Status, DiffIdentical)
	}
}

func TestDiffNew(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	if err := os.MkdirAll(filepath.Join(backupDir, "ssh"), 0755); err != nil { t.Fatal(err) }

	// Create backup file
	backupFilePath := filepath.Join(backupDir, "ssh", "config")
	if err := os.WriteFile(backupFilePath, []byte("config data"), 0644); err != nil {
		t.Fatal(err)
	}

	// System path points to a non-existent file
	nonExistentPath := filepath.Join(dir, "nonexistent", "config")

	manifest := &backup.Manifest{
		Categories: map[string]*backup.ManifestCategory{
			"ssh": {
				BackedUp:  true,
				FileCount: 1,
				Files: []backup.ManifestEntry{
					{
						Path:      "ssh/config",
						Original:  nonExistentPath,
						SHA256:    "somehash",
						Encrypted: false,
					},
				},
			},
		},
	}

	engine := NewEngine(&crypto.NullDecryptor{}, logger.New(false))
	diffs, err := engine.Diff(context.Background(), manifest, backupDir, nil)
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1", len(diffs))
	}
	if diffs[0].Status != DiffNew {
		t.Errorf("status = %q, want %q", diffs[0].Status, DiffNew)
	}
}

func TestDiffMissing(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	if err := os.MkdirAll(backupDir, 0755); err != nil { t.Fatal(err) }

	// Do NOT create the backup file - it should be "missing"
	systemFilePath := filepath.Join(dir, "system", "config")

	manifest := &backup.Manifest{
		Categories: map[string]*backup.ManifestCategory{
			"ssh": {
				BackedUp:  true,
				FileCount: 1,
				Files: []backup.ManifestEntry{
					{
						Path:      "ssh/config",
						Original:  systemFilePath,
						SHA256:    "somehash",
						Encrypted: false,
					},
				},
			},
		},
	}

	engine := NewEngine(&crypto.NullDecryptor{}, logger.New(false))
	diffs, err := engine.Diff(context.Background(), manifest, backupDir, nil)
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1", len(diffs))
	}
	if diffs[0].Status != DiffMissing {
		t.Errorf("status = %q, want %q", diffs[0].Status, DiffMissing)
	}
}

func TestDiffModified(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	systemDir := filepath.Join(dir, "system")
	if err := os.MkdirAll(filepath.Join(backupDir, "ssh"), 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(systemDir, 0755); err != nil { t.Fatal(err) }

	// Create backup file with original content
	backupFilePath := filepath.Join(backupDir, "ssh", "config")
	if err := os.WriteFile(backupFilePath, []byte("original content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create system file with DIFFERENT content
	systemFilePath := filepath.Join(systemDir, "config")
	if err := os.WriteFile(systemFilePath, []byte("modified content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Hash of the ORIGINAL content (backup)
	backupHash, err := fsutil.SHA256File(backupFilePath)
	if err != nil {
		t.Fatal(err)
	}

	manifest := &backup.Manifest{
		Categories: map[string]*backup.ManifestCategory{
			"ssh": {
				BackedUp:  true,
				FileCount: 1,
				Files: []backup.ManifestEntry{
					{
						Path:      "ssh/config",
						Original:  systemFilePath,
						SHA256:    backupHash,
						Encrypted: false,
					},
				},
			},
		},
	}

	engine := NewEngine(&crypto.NullDecryptor{}, logger.New(false))
	diffs, err := engine.Diff(context.Background(), manifest, backupDir, nil)
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1", len(diffs))
	}
	if diffs[0].Status != DiffModified {
		t.Errorf("status = %q, want %q", diffs[0].Status, DiffModified)
	}
}

func TestDiffWithCategoryFilter(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	if err := os.MkdirAll(filepath.Join(backupDir, "ssh"), 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(filepath.Join(backupDir, "git"), 0755); err != nil { t.Fatal(err) }

	// Create backup files
	if err := os.WriteFile(filepath.Join(backupDir, "ssh", "config"), []byte("ssh config"), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(backupDir, "git", "gitconfig"), []byte("git config"), 0644); err != nil { t.Fatal(err) }

	manifest := &backup.Manifest{
		Categories: map[string]*backup.ManifestCategory{
			"ssh": {
				BackedUp:  true,
				FileCount: 1,
				Files: []backup.ManifestEntry{
					{
						Path:     "ssh/config",
						Original: filepath.Join(dir, "nonexistent", "ssh_config"),
						SHA256:   "hash1",
					},
				},
			},
			"git": {
				BackedUp:  true,
				FileCount: 1,
				Files: []backup.ManifestEntry{
					{
						Path:     "git/gitconfig",
						Original: filepath.Join(dir, "nonexistent", "gitconfig"),
						SHA256:   "hash2",
					},
				},
			},
		},
	}

	engine := NewEngine(&crypto.NullDecryptor{}, logger.New(false))

	// Filter to only ssh
	diffs, err := engine.Diff(context.Background(), manifest, backupDir, []string{"ssh"})
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1 (only ssh)", len(diffs))
	}
	if diffs[0].Category != "ssh" {
		t.Errorf("category = %q, want %q", diffs[0].Category, "ssh")
	}
}

func TestDiffSkipsNotBackedUp(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	if err := os.MkdirAll(backupDir, 0755); err != nil { t.Fatal(err) }

	manifest := &backup.Manifest{
		Categories: map[string]*backup.ManifestCategory{
			"ssh": {
				BackedUp:  false,
				FileCount: 1,
				Files: []backup.ManifestEntry{
					{
						Path:     "ssh/config",
						Original: "/tmp/doesnotmatter",
						SHA256:   "hash",
					},
				},
			},
		},
	}

	engine := NewEngine(&crypto.NullDecryptor{}, logger.New(false))
	diffs, err := engine.Diff(context.Background(), manifest, backupDir, nil)
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}

	if len(diffs) != 0 {
		t.Errorf("got %d diffs, want 0 (category not backed up)", len(diffs))
	}
}
