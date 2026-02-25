package fsutil

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const (
	destZipFile     = "out.zip"
	errZipDir       = "ZipDir() error: %v"
	errUnzipToTemp  = "UnzipToTemp() error: %v"
	errOpeningZip   = "opening zip: %v"
	testFileName    = "file.txt"
)

// makeTestTree creates a directory tree for zip tests.
// Returns a map of relPath → content for verification.
func makeTestTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{
		"manifest.yaml":             []byte("version: 1\n"),
		"ssh/config":                []byte("Host *\n  ServerAliveInterval 60\n"),
		"ssh/id_rsa.age":            []byte("age-encrypted-content"),
		"browser/Default/Bookmarks": []byte(`{"roots":{}}`),
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// TestZipDirRoundTrip zips a directory, extracts it, and verifies all files are identical.
func TestZipDirRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	destZip := filepath.Join(dir, destZipFile)

	want := makeTestTree(t, src)

	if err := ZipDir(src, destZip); err != nil {
		t.Fatalf(errZipDir, err)
	}

	if !FileExists(destZip) {
		t.Fatal("zip file not created")
	}

	extracted, cleanup, err := UnzipToTemp(destZip)
	if err != nil {
		t.Fatalf(errUnzipToTemp, err)
	}
	defer cleanup()

	for rel, wantContent := range want {
		got, err := os.ReadFile(filepath.Join(extracted, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("file %s missing after unzip: %v", rel, err)
			continue
		}
		if !bytes.Equal(got, wantContent) {
			t.Errorf("file %s content mismatch: got %q, want %q", rel, got, wantContent)
		}
	}
}

// TestZipDirPreservesNestedPaths verifies nested paths are stored with forward slashes.
func TestZipDirPreservesNestedPaths(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")

	if err := os.MkdirAll(filepath.Join(src, "a", "b"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "b", "c.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	destZip := filepath.Join(dir, destZipFile)
	if err := ZipDir(src, destZip); err != nil {
		t.Fatalf(errZipDir, err)
	}

	r, err := zip.OpenReader(destZip)
	if err != nil {
		t.Fatalf(errOpeningZip, err)
	}
	defer r.Close()

	found := false
	for _, f := range r.File {
		if f.Name == "a/b/c.txt" {
			found = true
		}
	}
	if !found {
		t.Error("nested path a/b/c.txt not found in zip with forward slashes")
	}
}

// TestZipDirSkipsSymlinks verifies symlinks are not included in the zip.
func TestZipDirSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	// Real file
	if err := os.WriteFile(filepath.Join(src, "real.txt"), []byte("real"), 0644); err != nil {
		t.Fatal(err)
	}
	// Symlink
	if err := os.Symlink(filepath.Join(src, "real.txt"), filepath.Join(src, "link.txt")); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	destZip := filepath.Join(dir, destZipFile)
	if err := ZipDir(src, destZip); err != nil {
		t.Fatalf(errZipDir, err)
	}

	r, err := zip.OpenReader(destZip)
	if err != nil {
		t.Fatalf(errOpeningZip, err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "link.txt" {
			t.Error("symlink link.txt should not be included in zip")
		}
	}
}

// TestZipDirAtomicWrite verifies no partial .zip artifact remains if ZipDir is called
// on a non-existent source (walk fails immediately).
func TestZipDirAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	destZip := filepath.Join(dir, destZipFile)

	// srcDir does not exist → WalkDir returns error → atomic write should clean up
	err := ZipDir(filepath.Join(dir, "nonexistent"), destZip)
	if err == nil {
		t.Fatal("ZipDir on missing src should return error")
	}

	if FileExists(destZip) {
		t.Error("partial zip file should not exist after error")
	}
	// Also verify no leftover .macback-zip-* temp file
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if len(e.Name()) > 12 && e.Name()[:13] == ".macback-zip-" {
			t.Errorf("temp file %s left behind after error", e.Name())
		}
	}
}

// TestZipDirEmptyDir verifies an empty source produces a valid (entry-less) zip.
func TestZipDirEmptyDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "empty")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}

	destZip := filepath.Join(dir, destZipFile)
	if err := ZipDir(src, destZip); err != nil {
		t.Fatalf("ZipDir() on empty dir error: %v", err)
	}

	r, err := zip.OpenReader(destZip)
	if err != nil {
		t.Fatalf(errOpeningZip, err)
	}
	defer r.Close()

	if len(r.File) != 0 {
		t.Errorf("expected 0 entries in empty zip, got %d", len(r.File))
	}
}

// TestZipDirOverwritesExisting verifies calling ZipDir twice replaces the first zip.
func TestZipDirOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, testFileName), []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	destZip := filepath.Join(dir, destZipFile)
	if err := ZipDir(src, destZip); err != nil {
		t.Fatalf("first ZipDir() error: %v", err)
	}

	// Update source and zip again
	if err := os.WriteFile(filepath.Join(src, testFileName), []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ZipDir(src, destZip); err != nil {
		t.Fatalf("second ZipDir() error: %v", err)
	}

	extracted, cleanup, err := UnzipToTemp(destZip)
	if err != nil {
		t.Fatalf(errUnzipToTemp, err)
	}
	defer cleanup()

	got, _ := os.ReadFile(filepath.Join(extracted, testFileName))
	if string(got) != "v2" {
		t.Errorf("expected v2 after overwrite, got %q", got)
	}
}

// TestUnzipToTempExtractsFiles verifies files are extracted correctly.
func TestUnzipToTempExtractsFiles(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	want := makeTestTree(t, src)

	destZip := filepath.Join(dir, destZipFile)
	if err := ZipDir(src, destZip); err != nil {
		t.Fatalf(errZipDir, err)
	}

	extracted, cleanup, err := UnzipToTemp(destZip)
	if err != nil {
		t.Fatalf(errUnzipToTemp, err)
	}
	defer cleanup()

	for rel := range want {
		if !FileExists(filepath.Join(extracted, filepath.FromSlash(rel))) {
			t.Errorf("file %s missing after extraction", rel)
		}
	}
}

// TestUnzipToTempNestedDirs verifies nested directories are created.
func TestUnzipToTempNestedDirs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "a", "b", "c"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "b", "c", "deep.txt"), []byte("deep"), 0644); err != nil {
		t.Fatal(err)
	}

	destZip := filepath.Join(dir, destZipFile)
	if err := ZipDir(src, destZip); err != nil {
		t.Fatalf(errZipDir, err)
	}

	extracted, cleanup, err := UnzipToTemp(destZip)
	if err != nil {
		t.Fatalf(errUnzipToTemp, err)
	}
	defer cleanup()

	if !FileExists(filepath.Join(extracted, "a", "b", "c", "deep.txt")) {
		t.Error("deeply nested file not extracted")
	}
}

// TestUnzipToTempZipSlipRejected verifies entries with ".." are rejected.
func TestUnzipToTempZipSlipRejected(t *testing.T) {
	dir := t.TempDir()
	destZip := filepath.Join(dir, "evil.zip")

	// Craft a zip with a path-traversal entry
	f, err := os.Create(destZip)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../../evil.sh")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("rm -rf /"))
	_ = zw.Close()
	_ = f.Close()

	_, cleanup, err := UnzipToTemp(destZip)
	if err == nil {
		cleanup()
		t.Error("UnzipToTemp() should reject zip-slip entry")
	}
}

// TestUnzipToTempCleanupRemovesDir verifies cleanup() deletes the temp dir.
func TestUnzipToTempCleanupRemovesDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	destZip := filepath.Join(dir, destZipFile)
	if err := ZipDir(src, destZip); err != nil {
		t.Fatalf(errZipDir, err)
	}

	extracted, cleanup, err := UnzipToTemp(destZip)
	if err != nil {
		t.Fatalf(errUnzipToTemp, err)
	}
	if !DirExists(extracted) {
		t.Fatal("temp dir should exist before cleanup")
	}

	cleanup()

	if DirExists(extracted) {
		t.Error("temp dir should be removed after cleanup()")
	}
}

// TestUnzipToTempInvalidZip verifies a non-zip file returns an error.
func TestUnzipToTempInvalidZip(t *testing.T) {
	dir := t.TempDir()
	notZip := filepath.Join(dir, "notazip.zip")
	if err := os.WriteFile(notZip, []byte("this is not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	_, cleanup, err := UnzipToTemp(notZip)
	if err == nil {
		cleanup()
		t.Error("UnzipToTemp() should return error for invalid zip file")
	}
}
