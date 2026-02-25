package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hiiamtrong/macback/internal/config"
	"github.com/hiiamtrong/macback/internal/crypto"
	"github.com/hiiamtrong/macback/internal/fsutil"
)

const testProfile1 = "Profile 1"

// defaultBrowserCfg returns a CategoryConfig suitable for browser tests.
func defaultBrowserCfg() *config.CategoryConfig {
	return &config.CategoryConfig{
		Enabled:       true,
		MaxFileSizeMB: 50,
		Exclude:       []string{},
	}
}

// makeBrowserFile is a helper that creates a file under a profile dir.
func makeBrowserFile(t *testing.T, profileDir, relPath string, content []byte) {
	t.Helper()
	makeFile(t, filepath.Join(profileDir, relPath), content)
}

// --- Unit tests for helpers ---

// T3.9
func TestIsBuiltinCacheDir(t *testing.T) {
	caches := []string{
		"Cache", "Code Cache", "GPUCache", "DawnCache",
		"DawnGraphiteCache", "GraphiteDawnCache", "GrShaderCache",
		"CacheStorage", "ScriptCache", "PnaclTranslationCache", "ShaderCache",
	}
	for _, name := range caches {
		if !isBuiltinCacheDir(name) {
			t.Errorf("isBuiltinCacheDir(%q) = false, want true", name)
		}
	}
	notCaches := []string{"Bookmarks", "Preferences", "History", "Extensions", "Default"}
	for _, name := range notCaches {
		if isBuiltinCacheDir(name) {
			t.Errorf("isBuiltinCacheDir(%q) = true, want false", name)
		}
	}
}

// T3.10
func TestDiscoverProfiles(t *testing.T) {
	dir := t.TempDir()

	// Valid profile dirs
	for _, name := range []string{"Default", testProfile1, "Profile 2", "Guest Profile"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Non-profile dirs — should NOT be returned
	for _, name := range []string{"Crashpad", "NativeMessagingHosts", "GrShaderCache"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}

	profiles := discoverProfiles(dir)
	if len(profiles) != 4 {
		t.Fatalf("discoverProfiles returned %d profiles, want 4: %v", len(profiles), profiles)
	}

	profileNames := make(map[string]bool)
	for _, p := range profiles {
		profileNames[filepath.Base(p)] = true
	}
	for _, expected := range []string{"Default", testProfile1, "Profile 2", "Guest Profile"} {
		if !profileNames[expected] {
			t.Errorf("expected profile %q not found in results", expected)
		}
	}
}

func TestDiscoverProfiles_MissingDir(t *testing.T) {
	profiles := discoverProfiles("/nonexistent/path")
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles for missing dir, got %d", len(profiles))
	}
}

// --- discoverBrowserFiles tests (lower-level, no system browser interference) ---

// T3.1
func TestBrowserDiscover_DetectsBrowser(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "Default")

	makeBrowserFile(t, profileDir, "Bookmarks", []byte(`{"roots":{}}`))
	makeBrowserFile(t, profileDir, "Preferences", []byte(`{}`))

	cfg := defaultBrowserCfg()
	entries := discoverBrowserFiles("MyBrowser", profileDir, cfg)

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entryRelPaths(entries))
	}

	paths := entryRelPaths(entries)
	wantPaths := []string{
		filepath.Join("browser", "MyBrowser", "Default", "Bookmarks"),
		filepath.Join("browser", "MyBrowser", "Default", "Preferences"),
	}
	for i, want := range wantPaths {
		if paths[i] != want {
			t.Errorf("RelPath[%d] = %q, want %q", i, paths[i], want)
		}
	}

	for _, e := range entries {
		if e.Category != "browser" {
			t.Errorf("entry.Category = %q, want %q", e.Category, "browser")
		}
	}
}

// T3.2
func TestBrowserDiscover_ExcludesCacheDirs(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "Default")

	// A real file that should be included
	makeBrowserFile(t, profileDir, "Bookmarks", []byte(`{"roots":{}}`))

	// Cache dirs that should be excluded
	for _, cacheDir := range []string{"Cache", "Code Cache", "GPUCache", "DawnCache", "ShaderCache"} {
		makeBrowserFile(t, profileDir, filepath.Join(cacheDir, "f_000001"), []byte("cached"))
	}

	cfg := defaultBrowserCfg()
	entries := discoverBrowserFiles("MyBrowser", profileDir, cfg)

	// Only Bookmarks should appear
	paths := entryRelPaths(entries)
	if len(paths) != 1 {
		t.Fatalf("expected 1 entry (Bookmarks), got %d: %v", len(paths), paths)
	}
	want := filepath.Join("browser", "MyBrowser", "Default", "Bookmarks")
	if paths[0] != want {
		t.Errorf("RelPath = %q, want %q", paths[0], want)
	}
}

// T3.3 — verifies both profiles are discovered via discoverProfiles + discoverBrowserFiles
func TestBrowserDiscover_MultipleProfiles(t *testing.T) {
	dir := t.TempDir()

	defaultProfile := filepath.Join(dir, "Default")
	profile1 := filepath.Join(dir, testProfile1)
	makeBrowserFile(t, defaultProfile, "Bookmarks", []byte(`{}`))
	makeBrowserFile(t, profile1, "Bookmarks", []byte(`{}`))

	cfg := defaultBrowserCfg()

	var entries []FileEntry
	for _, profile := range discoverProfiles(dir) {
		entries = append(entries, discoverBrowserFiles("Chrome", profile, cfg)...)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (one per profile), got %d: %v", len(entries), entryRelPaths(entries))
	}

	paths := entryRelPaths(entries)
	wantPaths := []string{
		filepath.Join("browser", "Chrome", "Default", "Bookmarks"),
		filepath.Join("browser", "Chrome", testProfile1, "Bookmarks"),
	}
	for i, want := range wantPaths {
		if paths[i] != want {
			t.Errorf("RelPath[%d] = %q, want %q", i, paths[i], want)
		}
	}
}

// T3.4
func TestBrowserDiscover_MissingBrowserSkipped(t *testing.T) {
	profiles := discoverProfiles("/nonexistent/browser/path")
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles for missing browser dir, got %d", len(profiles))
	}
}

// T3.5
func TestBrowserDiscover_UserExclude(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "Default")

	makeBrowserFile(t, profileDir, "Bookmarks", []byte(`{}`))
	makeBrowserFile(t, profileDir, "History", []byte(`{}`))
	makeBrowserFile(t, profileDir, "Preferences", []byte(`{}`))

	cfg := defaultBrowserCfg()
	cfg.Exclude = []string{"History"} // user excludes History

	entries := discoverBrowserFiles("Brave", profileDir, cfg)
	paths := entryRelPaths(entries)

	for _, p := range paths {
		if filepath.Base(p) == "History" {
			t.Errorf("History should be excluded, but found: %q", p)
		}
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 entries (Bookmarks + Preferences), got %d: %v", len(paths), paths)
	}
}

// T3.6
func TestBrowserDiscover_SizeFilter(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "Default")

	// Small file — should be included
	makeBrowserFile(t, profileDir, "Bookmarks", []byte(`{}`))

	// Large file — sparse 60 MB
	largePath := filepath.Join(profileDir, "large.bin")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(largePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(60 * 1024 * 1024); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()

	cfg := defaultBrowserCfg()
	cfg.MaxFileSizeMB = 50

	entries := discoverBrowserFiles("Brave", profileDir, cfg)
	paths := entryRelPaths(entries)

	if len(paths) != 1 {
		t.Fatalf("expected 1 entry (Bookmarks), got %d: %v", len(paths), paths)
	}
	if filepath.Base(paths[0]) != "Bookmarks" {
		t.Errorf("expected Bookmarks, got %q", paths[0])
	}
}

// --- Backup tests (FileEntries constructed directly) ---

// T3.7
func TestBrowserBackup_CopiesFiles(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "src", "Default")
	destDir := filepath.Join(dir, "backup")

	makeBrowserFile(t, profileDir, "Bookmarks", []byte(`{"roots":{}}`))
	makeBrowserFile(t, profileDir, "Preferences", []byte(`{}`))

	cfg := defaultBrowserCfg()
	entries := discoverBrowserFiles("Brave", profileDir, cfg)

	if len(entries) != 2 {
		t.Fatalf("Discover returned %d entries, want 2", len(entries))
	}

	h := &BrowserHandler{}
	result, err := h.Backup(context.Background(), entries, destDir, &crypto.NullEncryptor{})
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}

	if result.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", result.FileCount)
	}
	if result.EncryptedCount != 0 {
		t.Errorf("EncryptedCount = %d, want 0", result.EncryptedCount)
	}

	// Verify files exist at expected RelPaths under destDir
	bookmarksPath := filepath.Join(destDir, "browser", "Brave", "Default", "Bookmarks")
	preferencesPath := filepath.Join(destDir, "browser", "Brave", "Default", "Preferences")
	if !fsutil.FileExists(bookmarksPath) {
		t.Errorf("Bookmarks not found at expected path: %s", bookmarksPath)
	}
	if !fsutil.FileExists(preferencesPath) {
		t.Errorf("Preferences not found at expected path: %s", preferencesPath)
	}
}

// T3.8
func TestBrowserBackup_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "Default")

	makeBrowserFile(t, profileDir, "Bookmarks", []byte(`{}`))

	cfg := defaultBrowserCfg()
	entries := discoverBrowserFiles("Brave", profileDir, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	h := &BrowserHandler{}
	_, err := h.Backup(ctx, entries, t.TempDir(), &crypto.NullEncryptor{})
	if err == nil {
		t.Error("Backup() should return error when context is cancelled")
	}
}
