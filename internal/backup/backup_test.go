package backup

import (
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/trongdev/macos-backup/internal/crypto"
	"github.com/trongdev/macos-backup/internal/fsutil"
)

func TestBackupFileEntry(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst")
	if err := os.MkdirAll(srcDir, 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(dstDir, 0755); err != nil { t.Fatal(err) }

	content := []byte("hello backup test")
	srcPath := filepath.Join(srcDir, "testfile.txt")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(srcPath)
	if err != nil {
		t.Fatal(err)
	}

	entry := FileEntry{
		SourcePath: srcPath,
		RelPath:    "testfile.txt",
		Category:   "test",
		IsSecret:   false,
		Mode:       info.Mode().Perm(),
		ModTime:    info.ModTime(),
		Size:       info.Size(),
	}

	enc := &crypto.NullEncryptor{}
	me, err := BackupFileEntry(entry, dstDir, enc)
	if err != nil {
		t.Fatalf("BackupFileEntry() error: %v", err)
	}

	// Verify ManifestEntry has correct fields
	if me.Original == "" {
		t.Error("ManifestEntry.Original should not be empty")
	}
	if me.Mode != fsutil.FileModeString(info.Mode().Perm()) {
		t.Errorf("Mode = %q, want %q", me.Mode, fsutil.FileModeString(info.Mode().Perm()))
	}
	if me.SHA256 == "" {
		t.Error("SHA256 should not be empty")
	}
	if me.Encrypted {
		t.Error("Encrypted should be false for NullEncryptor")
	}

	// Verify destination file was created
	dstPath := filepath.Join(dstDir, "testfile.txt")
	if !fsutil.FileExists(dstPath) {
		t.Error("destination file should exist")
	}

	// Verify SHA256 matches source
	srcHash, err := fsutil.SHA256File(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	if me.SHA256 != srcHash {
		t.Errorf("SHA256 = %q, want %q", me.SHA256, srcHash)
	}

	// Verify destination content matches source
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(dstContent) != string(content) {
		t.Errorf("destination content = %q, want %q", dstContent, content)
	}
}

func TestBackupFileEntryEncrypted(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst")
	if err := os.MkdirAll(srcDir, 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(dstDir, 0755); err != nil { t.Fatal(err) }

	content := []byte("secret content for encryption")
	srcPath := filepath.Join(srcDir, "secret.txt")
	if err := os.WriteFile(srcPath, content, 0600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(srcPath)
	if err != nil {
		t.Fatal(err)
	}

	// Compute source hash before backup
	srcHash, err := fsutil.SHA256File(srcPath)
	if err != nil {
		t.Fatal(err)
	}

	entry := FileEntry{
		SourcePath: srcPath,
		RelPath:    "secret.txt",
		Category:   "test",
		IsSecret:   true,
		Mode:       info.Mode().Perm(),
		ModTime:    info.ModTime(),
		Size:       info.Size(),
	}

	enc := crypto.NewPassphraseEncryptor("test-passphrase")
	me, err := BackupFileEntry(entry, dstDir, enc)
	if err != nil {
		t.Fatalf("BackupFileEntry() error: %v", err)
	}

	// Verify ManifestEntry.Encrypted is true
	if !me.Encrypted {
		t.Error("Encrypted should be true for PassphraseEncryptor")
	}

	// Verify destination file has .age extension
	encPath := filepath.Join(dstDir, "secret.txt.age")
	if !fsutil.FileExists(encPath) {
		t.Error("encrypted destination file with .age extension should exist")
	}

	// Verify original SHA256 is preserved (hash of the original plaintext)
	if me.SHA256 != srcHash {
		t.Errorf("SHA256 = %q, want %q (original source hash)", me.SHA256, srcHash)
	}

	// Verify encrypted content differs from original
	encContent, err := os.ReadFile(encPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(encContent) == string(content) {
		t.Error("encrypted content should differ from original")
	}
}

func TestManifestWriteRead(t *testing.T) {
	dir := t.TempDir()

	original := &Manifest{
		Version:        1,
		CreatedAt:      time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Hostname:       "test-host",
		Username:       "testuser",
		MacOSVersion:   "15.0",
		MacbackVersion: "dev",
		Categories: map[string]*ManifestCategory{
			"ssh": {
				BackedUp:       true,
				FileCount:      2,
				EncryptedCount: 1,
				Files: []ManifestEntry{
					{
						Path:      "ssh/config",
						Original:  "~/.ssh/config",
						Size:      256,
						Mode:      "0644",
						ModTime:   time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC),
						SHA256:    "abc123def456",
						Encrypted: false,
					},
					{
						Path:      "ssh/id_rsa.age",
						Original:  "~/.ssh/id_rsa",
						Size:      1024,
						Mode:      "0600",
						ModTime:   time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC),
						SHA256:    "def456abc123",
						Encrypted: true,
					},
				},
			},
		},
	}

	// Write
	if err := WriteManifest(original, dir); err != nil {
		t.Fatalf("WriteManifest() error: %v", err)
	}

	// Verify manifest file was created
	manifestPath := filepath.Join(dir, "manifest.yaml")
	if !fsutil.FileExists(manifestPath) {
		t.Fatal("manifest.yaml should exist")
	}

	// Read back
	loaded, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest() error: %v", err)
	}

	// Verify all fields match
	if loaded.Version != original.Version {
		t.Errorf("Version = %d, want %d", loaded.Version, original.Version)
	}
	if !loaded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", loaded.CreatedAt, original.CreatedAt)
	}
	if loaded.Hostname != original.Hostname {
		t.Errorf("Hostname = %q, want %q", loaded.Hostname, original.Hostname)
	}
	if loaded.Username != original.Username {
		t.Errorf("Username = %q, want %q", loaded.Username, original.Username)
	}
	if loaded.MacOSVersion != original.MacOSVersion {
		t.Errorf("MacOSVersion = %q, want %q", loaded.MacOSVersion, original.MacOSVersion)
	}
	if loaded.MacbackVersion != original.MacbackVersion {
		t.Errorf("MacbackVersion = %q, want %q", loaded.MacbackVersion, original.MacbackVersion)
	}

	// Verify category data
	sshCat, ok := loaded.Categories["ssh"]
	if !ok {
		t.Fatal("ssh category should exist")
	}
	if !sshCat.BackedUp {
		t.Error("ssh BackedUp should be true")
	}
	if sshCat.FileCount != 2 {
		t.Errorf("ssh FileCount = %d, want 2", sshCat.FileCount)
	}
	if sshCat.EncryptedCount != 1 {
		t.Errorf("ssh EncryptedCount = %d, want 1", sshCat.EncryptedCount)
	}
	if len(sshCat.Files) != 2 {
		t.Fatalf("ssh Files count = %d, want 2", len(sshCat.Files))
	}

	// Verify file entries
	f0 := sshCat.Files[0]
	if f0.Path != "ssh/config" {
		t.Errorf("Files[0].Path = %q, want %q", f0.Path, "ssh/config")
	}
	if f0.Original != "~/.ssh/config" {
		t.Errorf("Files[0].Original = %q, want %q", f0.Original, "~/.ssh/config")
	}
	if f0.SHA256 != "abc123def456" {
		t.Errorf("Files[0].SHA256 = %q, want %q", f0.SHA256, "abc123def456")
	}
	if f0.Encrypted {
		t.Error("Files[0].Encrypted should be false")
	}

	f1 := sshCat.Files[1]
	if !f1.Encrypted {
		t.Error("Files[1].Encrypted should be true")
	}
	if f1.Mode != "0600" {
		t.Errorf("Files[1].Mode = %q, want %q", f1.Mode, "0600")
	}
}

func TestNewManifest(t *testing.T) {
	before := time.Now()
	m := NewManifest("1.0.0")
	after := time.Now()

	// Verify hostname is set
	expectedHostname, _ := os.Hostname()
	if m.Hostname != expectedHostname {
		t.Errorf("Hostname = %q, want %q", m.Hostname, expectedHostname)
	}

	// Verify username is set
	u, err := user.Current()
	if err == nil {
		if m.Username != u.Username {
			t.Errorf("Username = %q, want %q", m.Username, u.Username)
		}
	}

	// Verify version
	if m.MacbackVersion != "1.0.0" {
		t.Errorf("MacbackVersion = %q, want %q", m.MacbackVersion, "1.0.0")
	}

	// Verify manifest version
	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}

	// Verify created_at is within the expected range
	if m.CreatedAt.Before(before) || m.CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v, should be between %v and %v", m.CreatedAt, before, after)
	}

	// Verify categories map is initialized
	if m.Categories == nil {
		t.Error("Categories should be initialized")
	}
}

func TestManifestTotalFiles(t *testing.T) {
	tests := []struct {
		name       string
		categories map[string]*ManifestCategory
		want       int
	}{
		{
			name:       "empty manifest",
			categories: map[string]*ManifestCategory{},
			want:       0,
		},
		{
			name: "single category",
			categories: map[string]*ManifestCategory{
				"ssh": {FileCount: 5},
			},
			want: 5,
		},
		{
			name: "multiple categories",
			categories: map[string]*ManifestCategory{
				"ssh":      {FileCount: 3},
				"dotfiles": {FileCount: 10},
				"git":      {FileCount: 2},
			},
			want: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manifest{Categories: tt.categories}
			got := m.TotalFiles()
			if got != tt.want {
				t.Errorf("TotalFiles() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestManifestHasEncryptedFiles(t *testing.T) {
	tests := []struct {
		name       string
		categories map[string]*ManifestCategory
		want       bool
	}{
		{
			name:       "empty manifest",
			categories: map[string]*ManifestCategory{},
			want:       false,
		},
		{
			name: "no encrypted files",
			categories: map[string]*ManifestCategory{
				"ssh":      {EncryptedCount: 0},
				"dotfiles": {EncryptedCount: 0},
			},
			want: false,
		},
		{
			name: "has encrypted files",
			categories: map[string]*ManifestCategory{
				"ssh":      {EncryptedCount: 2},
				"dotfiles": {EncryptedCount: 0},
			},
			want: true,
		},
		{
			name: "all categories have encrypted files",
			categories: map[string]*ManifestCategory{
				"ssh":      {EncryptedCount: 1},
				"dotfiles": {EncryptedCount: 3},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manifest{Categories: tt.categories}
			got := m.HasEncryptedFiles()
			if got != tt.want {
				t.Errorf("HasEncryptedFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackupFileEntryCreatesDestDir(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst", "nested", "dir")
	if err := os.MkdirAll(srcDir, 0755); err != nil { t.Fatal(err) }

	content := []byte("nested test")
	srcPath := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(srcPath)
	if err != nil {
		t.Fatal(err)
	}

	entry := FileEntry{
		SourcePath: srcPath,
		RelPath:    "file.txt",
		Category:   "test",
		Mode:       info.Mode().Perm(),
		ModTime:    info.ModTime(),
		Size:       info.Size(),
	}

	enc := &crypto.NullEncryptor{}
	_, err = BackupFileEntry(entry, dstDir, enc)
	if err != nil {
		t.Fatalf("BackupFileEntry() error: %v", err)
	}

	dstPath := filepath.Join(dstDir, "file.txt")
	if !fsutil.FileExists(dstPath) {
		t.Error("destination file should exist even with nested dir")
	}
}

func TestBackupFileEntryPreservesMode(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst")
	if err := os.MkdirAll(srcDir, 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(dstDir, 0755); err != nil { t.Fatal(err) }

	srcPath := filepath.Join(srcDir, "executable.sh")
	if err := os.WriteFile(srcPath, []byte("#!/bin/bash"), 0755); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(srcPath)
	if err != nil {
		t.Fatal(err)
	}

	entry := FileEntry{
		SourcePath: srcPath,
		RelPath:    "executable.sh",
		Category:   "test",
		Mode:       info.Mode().Perm(),
		ModTime:    info.ModTime(),
		Size:       info.Size(),
	}

	enc := &crypto.NullEncryptor{}
	me, err := BackupFileEntry(entry, dstDir, enc)
	if err != nil {
		t.Fatalf("BackupFileEntry() error: %v", err)
	}

	want := fsutil.FileModeString(fs.FileMode(0755))
	if me.Mode != want {
		t.Errorf("Mode = %q, want %q", me.Mode, want)
	}
}

func TestReadManifestNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadManifest(dir)
	if err == nil {
		t.Error("ReadManifest should return error for missing manifest")
	}
}
