package backup

import (
	"context"
	"encoding/json"
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
		filepath.Join("MyBrowser", "Default", "Bookmarks"),
		filepath.Join("MyBrowser", "Default", "Preferences"),
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
	want := filepath.Join("MyBrowser", "Default", "Bookmarks")
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
		filepath.Join("Chrome", "Default", "Bookmarks"),
		filepath.Join("Chrome", testProfile1, "Bookmarks"),
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
	bookmarksPath := filepath.Join(destDir, "Brave", "Default", "Bookmarks")
	preferencesPath := filepath.Join(destDir, "Brave", "Default", "Preferences")
	if !fsutil.FileExists(bookmarksPath) {
		t.Errorf("Bookmarks not found at expected path: %s", bookmarksPath)
	}
	if !fsutil.FileExists(preferencesPath) {
		t.Errorf("Preferences not found at expected path: %s", preferencesPath)
	}
}

// T3.11 — discoverBrowserRootFiles picks up Local State from the browser dir root.
func TestDiscoverBrowserRootFiles_LocalState(t *testing.T) {
	dir := t.TempDir()

	// Create Local State file at the browser dir root
	makeFile(t, filepath.Join(dir, "Local State"), []byte(`{"profile":{"info_cache":{}}}`))

	// Also create a profile dir with a file — should not appear in root files
	makeBrowserFile(t, filepath.Join(dir, "Default"), "Bookmarks", []byte(`{}`))

	entries := discoverBrowserRootFiles("Chrome", dir)

	if len(entries) != 1 {
		t.Fatalf("expected 1 root entry (Local State), got %d: %v", len(entries), entryRelPaths(entries))
	}
	want := filepath.Join("Chrome", "Local State")
	if entries[0].RelPath != want {
		t.Errorf("RelPath = %q, want %q", entries[0].RelPath, want)
	}
	if entries[0].Category != "browser" {
		t.Errorf("Category = %q, want %q", entries[0].Category, "browser")
	}
}

// T3.12 — discoverBrowserRootFiles returns nothing if Local State is absent.
func TestDiscoverBrowserRootFiles_Missing(t *testing.T) {
	dir := t.TempDir()
	entries := discoverBrowserRootFiles("Chrome", dir)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries when Local State absent, got %d", len(entries))
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

// TestPatchLocalStateContent_AddsUnregisteredProfiles verifies that profile dirs
// not present in info_cache are injected into the patched Local State.
func TestPatchLocalStateContent_AddsUnregisteredProfiles(t *testing.T) {
	dir := t.TempDir()

	// Local State with only Default registered
	localState := `{"profile":{"info_cache":{"Default":{"name":"Person 1"}}}}`
	localStatePath := filepath.Join(dir, "Local State")
	if err := os.WriteFile(localStatePath, []byte(localState), 0644); err != nil {
		t.Fatal(err)
	}

	// Profile 1 with a Preferences file (has a human-readable name)
	p1Dir := filepath.Join(dir, "Profile 1")
	if err := os.MkdirAll(p1Dir, 0755); err != nil {
		t.Fatal(err)
	}
	prefs := `{"profile":{"name":"Work"}}`
	if err := os.WriteFile(filepath.Join(p1Dir, "Preferences"), []byte(prefs), 0644); err != nil {
		t.Fatal(err)
	}

	// Profile 2 without a Preferences file (falls back to dir name)
	if err := os.MkdirAll(filepath.Join(dir, "Profile 2"), 0755); err != nil {
		t.Fatal(err)
	}

	patched, err := patchLocalStateContent(localStatePath, dir)
	if err != nil {
		t.Fatalf("patchLocalStateContent() error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(patched, &result); err != nil {
		t.Fatalf("patched JSON invalid: %v", err)
	}

	cache := result["profile"].(map[string]interface{})["info_cache"].(map[string]interface{})

	if _, ok := cache["Profile 1"]; !ok {
		t.Error("Profile 1 not added to info_cache")
	}
	if _, ok := cache["Profile 2"]; !ok {
		t.Error("Profile 2 not added to info_cache")
	}
	if _, ok := cache["Default"]; !ok {
		t.Error("Default should be preserved in info_cache")
	}

	// Verify Profile 1 got name from Preferences
	p1 := cache["Profile 1"].(map[string]interface{})
	if p1["name"] != "Work" {
		t.Errorf("Profile 1 name = %q, want %q", p1["name"], "Work")
	}
	// Profile 2 falls back to dir name
	p2 := cache["Profile 2"].(map[string]interface{})
	if p2["name"] != "Profile 2" {
		t.Errorf("Profile 2 name = %q, want %q", p2["name"], "Profile 2")
	}
}

// TestPatchLocalStateContent_NoChangeWhenAllRegistered verifies the function
// returns the original bytes unchanged when all profile dirs are already in info_cache.
func TestPatchLocalStateContent_NoChangeWhenAllRegistered(t *testing.T) {
	dir := t.TempDir()

	localState := `{"profile":{"info_cache":{"Default":{"name":"Person 1"}}}}`
	localStatePath := filepath.Join(dir, "Local State")
	if err := os.WriteFile(localStatePath, []byte(localState), 0644); err != nil {
		t.Fatal(err)
	}
	// No extra Profile N dirs — nothing to patch
	patched, err := patchLocalStateContent(localStatePath, dir)
	if err != nil {
		t.Fatalf("patchLocalStateContent() error: %v", err)
	}
	if string(patched) != localState {
		t.Errorf("expected unchanged output, got: %s", patched)
	}
}

// TestBackupBrowser_LocalStatePatched verifies that Backup() writes a patched
// Local State containing all discovered profile dirs.
func TestBackupBrowser_LocalStatePatched(t *testing.T) {
	browserDir := t.TempDir()
	destDir := t.TempDir()

	// Local State with only Default registered
	makeFile(t, filepath.Join(browserDir, "Local State"),
		[]byte(`{"profile":{"info_cache":{"Default":{"name":"Person 1"}}}}`))

	// Profile 1 dir with Preferences
	p1 := filepath.Join(browserDir, "Profile 1")
	if err := os.MkdirAll(p1, 0755); err != nil {
		t.Fatal(err)
	}
	makeFile(t, filepath.Join(p1, "Preferences"), []byte(`{"profile":{"name":"Work"}}`))
	makeFile(t, filepath.Join(p1, "Bookmarks"), []byte(`{}`))

	cfg := defaultBrowserCfg()
	entries := discoverBrowserRootFiles("Chrome", browserDir)
	entries = append(entries, discoverBrowserFiles("Chrome", p1, cfg)...)

	h := &BrowserHandler{}
	_, err := h.Backup(context.Background(), entries, destDir, &crypto.NullEncryptor{})
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}

	savedPath := filepath.Join(destDir, "Chrome", "Local State")
	data, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("saved Local State not found: %v", err)
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("saved Local State invalid JSON: %v", err)
	}
	cache := saved["profile"].(map[string]interface{})["info_cache"].(map[string]interface{})
	if _, ok := cache["Profile 1"]; !ok {
		t.Error("Profile 1 not injected into backed-up Local State")
	}
}
