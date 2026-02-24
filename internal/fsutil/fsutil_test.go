package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")

	content := []byte("hello world")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile() error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestCopyFilePreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")

	if err := os.WriteFile(src, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile() error: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %o, want %o", info.Mode().Perm(), 0600)
	}
}

func TestCopyFileCreatesDestDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "sub", "dir", "dest.txt")

	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile() error: %v", err)
	}

	if !FileExists(dst) {
		t.Error("destination file should exist")
	}
}

func TestCopyDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	// Create source structure
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("aaa"), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("bbb"), 0644); err != nil { t.Fatal(err) }

	if err := CopyDir(src, dst, nil); err != nil {
		t.Fatalf("CopyDir() error: %v", err)
	}

	// Check files exist
	if !FileExists(filepath.Join(dst, "a.txt")) {
		t.Error("a.txt should exist in destination")
	}
	if !FileExists(filepath.Join(dst, "sub", "b.txt")) {
		t.Error("sub/b.txt should exist in destination")
	}
}

func TestCopyDirWithExcludes(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	if err := os.MkdirAll(filepath.Join(src, "Cache"), 0755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(src, "skip.log"), []byte("skip"), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(src, "Cache", "data"), []byte("cache"), 0644); err != nil { t.Fatal(err) }

	if err := CopyDir(src, dst, []string{"*.log", "Cache"}); err != nil {
		t.Fatalf("CopyDir() error: %v", err)
	}

	if !FileExists(filepath.Join(dst, "keep.txt")) {
		t.Error("keep.txt should exist")
	}
	if FileExists(filepath.Join(dst, "skip.log")) {
		t.Error("skip.log should be excluded")
	}
	if DirExists(filepath.Join(dst, "Cache")) {
		t.Error("Cache/ should be excluded")
	}
}

func TestSHA256File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil { t.Fatal(err) }

	hash, err := SHA256File(path)
	if err != nil {
		t.Fatalf("SHA256File() error: %v", err)
	}

	// SHA256 of "hello" is known
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hash != expected {
		t.Errorf("SHA256 = %q, want %q", hash, expected)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil { t.Fatal(err) }

	if !FileExists(path) {
		t.Error("FileExists should return true for existing file")
	}
	if FileExists(filepath.Join(dir, "nope.txt")) {
		t.Error("FileExists should return false for missing file")
	}
	if FileExists(dir) {
		t.Error("FileExists should return false for directory")
	}
}

func TestDirExists(t *testing.T) {
	dir := t.TempDir()

	if !DirExists(dir) {
		t.Error("DirExists should return true for existing directory")
	}
	if DirExists(filepath.Join(dir, "nope")) {
		t.Error("DirExists should return false for missing directory")
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"tilde", "~", home},
		{"tilde slash", "~/foo", filepath.Join(home, "foo")},
		{"absolute", "/usr/bin", "/usr/bin"},
		{"relative", "foo/bar", "foo/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandPath(tt.input)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestContractPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"home", home, "~"},
		{"home subdir", home + "/Documents", "~/Documents"},
		{"other", "/usr/bin", "/usr/bin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContractPath(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExpandGlob(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(""), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte(""), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dir, "c.log"), []byte(""), 0644); err != nil { t.Fatal(err) }

	matches, err := ExpandGlob(filepath.Join(dir, "*.txt"))
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("got %d matches, want 2", len(matches))
	}
}

func TestFileModeString(t *testing.T) {
	if s := FileModeString(0644); s != "0644" {
		t.Errorf("got %q, want %q", s, "0644")
	}
	if s := FileModeString(0600); s != "0600" {
		t.Errorf("got %q, want %q", s, "0600")
	}
}

func TestParseFileMode(t *testing.T) {
	mode, err := ParseFileMode("0644")
	if err != nil {
		t.Fatal(err)
	}
	if mode != fs.FileMode(0644) {
		t.Errorf("got %o, want %o", mode, 0644)
	}
}
