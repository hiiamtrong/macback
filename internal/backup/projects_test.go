package backup

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hiiamtrong/macback/internal/config"
	"github.com/hiiamtrong/macback/internal/crypto"
	"github.com/hiiamtrong/macback/internal/fsutil"
)

const (
	testMainGo      = "main.go"
	testPackageMain = "package main"
	testIndexJS     = "index.js"
	testFileGo      = "file.go"
	errDiscoverFmt  = "Discover() error: %v"
	errRelPathFmt   = "RelPath = %q, want %q"
)

// --- helpers ---

func makeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
}

func entryRelPaths(entries []FileEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.RelPath)
	}
	sort.Strings(paths)
	return paths
}

func defaultProjectCfg() *config.CategoryConfig {
	return &config.CategoryConfig{
		Enabled:       true,
		ProjectDepth:  1,
		MaxFileSizeMB: 50,
		Exclude: []string{
			"node_modules", "vendor", ".venv", "venv", "env", "Pods",
			"__pycache__", ".gradle", "build", "target", "dist", ".next", ".nuxt", "out",
			".cache", ".git", ".DS_Store",
			"*.pyc", "*.class",
		},
	}
}

// --- Discover tests ---

func TestProjectsDiscover_BasicScan(t *testing.T) {
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "projects")
	projectDir := filepath.Join(scanDir, "myproject")

	makeFile(t, filepath.Join(projectDir, testMainGo), []byte(testPackageMain))
	makeFile(t, filepath.Join(projectDir, "README.md"), []byte("readme"))
	makeFile(t, filepath.Join(projectDir, "go.mod"), []byte("module example"))

	cfg := defaultProjectCfg()
	cfg.ScanDirs = []string{scanDir}

	h := &ProjectsHandler{}
	entries, err := h.Discover(cfg)
	if err != nil {
		t.Fatalf(errDiscoverFmt, err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(entries), entryRelPaths(entries))
	}

	for _, e := range entries {
		if e.Category != "projects" {
			t.Errorf("entry Category = %q, want %q", e.Category, "projects")
		}
		if e.SourcePath == "" {
			t.Error("SourcePath should not be empty")
		}
	}

	paths := entryRelPaths(entries)
	expected := []string{"myproject/README.md", "myproject/go.mod", "myproject/main.go"}
	for i, p := range expected {
		if paths[i] != p {
			t.Errorf("RelPath[%d] = %q, want %q", i, paths[i], p)
		}
	}
}

func TestProjectsDiscover_ExcludesNodeModules(t *testing.T) {
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "projects")
	projectDir := filepath.Join(scanDir, "webapp")

	makeFile(t, filepath.Join(projectDir, testIndexJS), []byte("console.log('hi')"))
	makeFile(t, filepath.Join(projectDir, "node_modules", "lodash", testIndexJS), []byte("lodash"))
	makeFile(t, filepath.Join(projectDir, "node_modules", "react", testIndexJS), []byte("react"))

	cfg := defaultProjectCfg()
	cfg.ScanDirs = []string{scanDir}

	h := &ProjectsHandler{}
	entries, err := h.Discover(cfg)
	if err != nil {
		t.Fatalf(errDiscoverFmt, err)
	}

	paths := entryRelPaths(entries)
	if len(paths) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(paths), paths)
	}
	if paths[0] != "webapp/index.js" {
		t.Errorf(errRelPathFmt, paths[0], "webapp/index.js")
	}
}

func TestProjectsDiscover_ExcludesAllEcosystems(t *testing.T) {
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "projects")
	projectDir := filepath.Join(scanDir, "myapp")

	// Source file — should be included
	makeFile(t, filepath.Join(projectDir, testMainGo), []byte(testPackageMain))

	// All ecosystem dirs — should be excluded
	excludedDirs := []string{
		"vendor", ".venv", "venv", "env", "Pods",
		"__pycache__", ".gradle", "build", "target", "dist", ".next", ".nuxt", "out",
		".cache", ".git",
	}
	for _, excDir := range excludedDirs {
		makeFile(t, filepath.Join(projectDir, excDir, "file.txt"), []byte("data"))
	}

	cfg := defaultProjectCfg()
	cfg.ScanDirs = []string{scanDir}

	h := &ProjectsHandler{}
	entries, err := h.Discover(cfg)
	if err != nil {
		t.Fatalf(errDiscoverFmt, err)
	}

	paths := entryRelPaths(entries)
	if len(paths) != 1 {
		t.Fatalf("expected only main.go, got %d entries: %v", len(paths), paths)
	}
	if paths[0] != "myapp/main.go" {
		t.Errorf(errRelPathFmt, paths[0], "myapp/main.go")
	}
}

func TestProjectsDiscover_SizeFilter(t *testing.T) {
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "projects")
	projectDir := filepath.Join(scanDir, "myapp")

	// Small file — 10 bytes, should be included
	makeFile(t, filepath.Join(projectDir, "small.txt"), []byte("0123456789"))

	// Large file — create sparse file of 60 MB
	largePath := filepath.Join(projectDir, "large.bin")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
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

	cfg := defaultProjectCfg()
	cfg.ScanDirs = []string{scanDir}
	cfg.MaxFileSizeMB = 50

	h := &ProjectsHandler{}
	entries, err := h.Discover(cfg)
	if err != nil {
		t.Fatalf(errDiscoverFmt, err)
	}

	paths := entryRelPaths(entries)
	if len(paths) != 1 {
		t.Fatalf("expected 1 entry (small.txt), got %d: %v", len(paths), paths)
	}
	if paths[0] != "myapp/small.txt" {
		t.Errorf(errRelPathFmt, paths[0], "myapp/small.txt")
	}
}

func TestProjectsDiscover_SizeFilterZeroMeansUnlimited(t *testing.T) {
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "projects")
	projectDir := filepath.Join(scanDir, "myapp")

	// Create a sparse 200 MB file
	largePath := filepath.Join(projectDir, "large.bin")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(largePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(200 * 1024 * 1024); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()

	cfg := defaultProjectCfg()
	cfg.ScanDirs = []string{scanDir}
	cfg.MaxFileSizeMB = 0 // zero = unlimited

	h := &ProjectsHandler{}
	entries, err := h.Discover(cfg)
	if err != nil {
		t.Fatalf(errDiscoverFmt, err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry with unlimited size, got %d", len(entries))
	}
}

func TestProjectsDiscover_MultipleScandirs(t *testing.T) {
	dir := t.TempDir()
	scanDir1 := filepath.Join(dir, "works")
	scanDir2 := filepath.Join(dir, "personal")

	makeFile(t, filepath.Join(scanDir1, "projectA", testMainGo), []byte(testPackageMain))
	makeFile(t, filepath.Join(scanDir2, "projectB", "app.py"), []byte("print('hi')"))

	cfg := defaultProjectCfg()
	cfg.ScanDirs = []string{scanDir1, scanDir2}

	h := &ProjectsHandler{}
	entries, err := h.Discover(cfg)
	if err != nil {
		t.Fatalf(errDiscoverFmt, err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries from 2 scan dirs, got %d: %v", len(entries), entryRelPaths(entries))
	}

	paths := entryRelPaths(entries)
	if paths[0] != "projectA/main.go" {
		t.Errorf("RelPath[0] = %q, want %q", paths[0], "projectA/main.go")
	}
	if paths[1] != "projectB/app.py" {
		t.Errorf("RelPath[1] = %q, want %q", paths[1], "projectB/app.py")
	}
}

func TestProjectsDiscover_MissingScandirSkipped(t *testing.T) {
	cfg := defaultProjectCfg()
	cfg.ScanDirs = []string{"/nonexistent/path/that/does/not/exist"}

	h := &ProjectsHandler{}
	entries, err := h.Discover(cfg)
	if err != nil {
		t.Fatalf("Discover() should not error on missing scan_dir, got: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for missing scan_dir, got %d", len(entries))
	}
}

func TestProjectsDiscover_ProjectDepth2(t *testing.T) {
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "works")

	// depth=2: works/company/projectA/file.go
	makeFile(t, filepath.Join(scanDir, "company", "projectA", testMainGo), []byte(testPackageMain))
	makeFile(t, filepath.Join(scanDir, "company", "projectB", testMainGo), []byte(testPackageMain))

	cfg := defaultProjectCfg()
	cfg.ScanDirs = []string{scanDir}
	cfg.ProjectDepth = 2

	h := &ProjectsHandler{}
	entries, err := h.Discover(cfg)
	if err != nil {
		t.Fatalf(errDiscoverFmt, err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries at depth=2, got %d: %v", len(entries), entryRelPaths(entries))
	}

	paths := entryRelPaths(entries)
	if paths[0] != "company/projectA/main.go" {
		t.Errorf("RelPath[0] = %q, want %q", paths[0], "company/projectA/main.go")
	}
	if paths[1] != "company/projectB/main.go" {
		t.Errorf("RelPath[1] = %q, want %q", paths[1], "company/projectB/main.go")
	}
}

// --- Backup tests ---

func TestProjectsBackup_CopiesFiles(t *testing.T) {
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "projects")
	projectDir := filepath.Join(scanDir, "myproject")
	destDir := filepath.Join(dir, "backup", "projects")

	makeFile(t, filepath.Join(projectDir, testMainGo), []byte(testPackageMain))
	makeFile(t, filepath.Join(projectDir, "utils", "helper.go"), []byte("package utils"))

	cfg := defaultProjectCfg()
	cfg.ScanDirs = []string{scanDir}

	h := &ProjectsHandler{}
	entries, err := h.Discover(cfg)
	if err != nil {
		t.Fatalf(errDiscoverFmt, err)
	}

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

	// Verify files exist in dest
	if !fsutil.FileExists(filepath.Join(destDir, "myproject", testMainGo)) {
		t.Error("myproject/main.go should exist in dest")
	}
	if !fsutil.FileExists(filepath.Join(destDir, "myproject", "utils", "helper.go")) {
		t.Error("myproject/utils/helper.go should exist in dest")
	}
}

func TestProjectsBackup_WarnsOnMissingSource(t *testing.T) {
	destDir := t.TempDir()

	entries := []FileEntry{
		{
			SourcePath: "/nonexistent/file.go",
			RelPath:    "proj/file.go",
			Category:   "projects",
		},
	}

	h := &ProjectsHandler{}
	result, err := h.Backup(context.Background(), entries, destDir, &crypto.NullEncryptor{})
	if err != nil {
		t.Fatalf("Backup() should not error, got: %v", err)
	}

	if result.FileCount != 0 {
		t.Errorf("FileCount = %d, want 0 (file should be skipped)", result.FileCount)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected at least one warning for missing source file")
	}
}

func TestProjectsBackup_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	makeFile(t, filepath.Join(dir, testFileGo), []byte(testPackageMain))

	info, _ := os.Stat(filepath.Join(dir, testFileGo))
	entries := []FileEntry{
		{
			SourcePath: filepath.Join(dir, testFileGo),
			RelPath:    "proj/file.go",
			Category:   "projects",
			Mode:       info.Mode().Perm(),
			ModTime:    info.ModTime(),
			Size:       info.Size(),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	h := &ProjectsHandler{}
	_, err := h.Backup(ctx, entries, dir, &crypto.NullEncryptor{})
	if err == nil {
		t.Error("Backup() should return error when context is cancelled")
	}
}

// --- Integration test ---

func TestProjectsBackup_Integration_NodeModulesAbsent(t *testing.T) {
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "projects")
	projectDir := filepath.Join(scanDir, "webapp")
	destDir := filepath.Join(dir, "backup", "projects")

	// Source files — should be backed up
	makeFile(t, filepath.Join(projectDir, "package.json"), []byte(`{"name":"webapp"}`))
	makeFile(t, filepath.Join(projectDir, "src", "App.tsx"), []byte("export default App"))
	makeFile(t, filepath.Join(projectDir, ".env"), []byte("SECRET=hunter2"))

	// node_modules — should NOT be backed up
	makeFile(t, filepath.Join(projectDir, "node_modules", "react", testIndexJS), []byte("react"))
	makeFile(t, filepath.Join(projectDir, "node_modules", "lodash", "lodash.js"), []byte("lodash"))

	cfg := defaultProjectCfg()
	cfg.ScanDirs = []string{scanDir}

	h := &ProjectsHandler{}
	entries, err := h.Discover(cfg)
	if err != nil {
		t.Fatalf(errDiscoverFmt, err)
	}

	result, err := h.Backup(context.Background(), entries, destDir, &crypto.NullEncryptor{})
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}

	// Should have 3 files: package.json, src/App.tsx, .env
	if result.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", result.FileCount)
	}

	// node_modules must not appear in dest
	nmPath := filepath.Join(destDir, "webapp", "node_modules")
	if fsutil.DirExists(nmPath) {
		t.Error("node_modules should NOT exist in backup dest")
	}

	// Source files must appear
	if !fsutil.FileExists(filepath.Join(destDir, "webapp", "package.json")) {
		t.Error("package.json should exist in dest")
	}
	if !fsutil.FileExists(filepath.Join(destDir, "webapp", "src", "App.tsx")) {
		t.Error("src/App.tsx should exist in dest")
	}
}
