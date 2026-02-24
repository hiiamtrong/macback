package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hiiamtrong/macback/internal/config"
)

func TestSSHDiscover(t *testing.T) {
	dir := t.TempDir()

	// Create fake SSH files
	files := map[string]string{
		"config":     "Host *\n  ServerAliveInterval 60",
		"id_rsa":     "-----BEGIN RSA PRIVATE KEY-----\nfake",
		"id_rsa.pub": "ssh-rsa AAAA fake-key",
		"known_hosts": "github.com ssh-rsa AAAA",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create agent.sock (should be excluded)
	if err := os.WriteFile(filepath.Join(dir, "agent.sock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &SSHHandler{}
	cfg := &config.CategoryConfig{
		Enabled: true,
		Paths:   []string{filepath.Join(dir, "*")},
		Exclude: []string{"*.sock"},
	}

	entries, err := handler.Discover(cfg)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	// Verify agent.sock is excluded
	foundNames := make(map[string]bool)
	for _, e := range entries {
		foundNames[e.RelPath] = true
	}

	if foundNames["agent.sock"] {
		t.Error("agent.sock should be excluded")
	}

	// Verify expected files are discovered
	expectedFiles := []string{"config", "id_rsa", "id_rsa.pub", "known_hosts"}
	for _, name := range expectedFiles {
		if !foundNames[name] {
			t.Errorf("expected file %q not found in discovered entries", name)
		}
	}

	if len(entries) != len(expectedFiles) {
		t.Errorf("discovered %d files, want %d", len(entries), len(expectedFiles))
	}
}

func TestDotfilesDiscoverExcludes(t *testing.T) {
	dir := t.TempDir()

	// Create files to keep
	if err := os.WriteFile(filepath.Join(dir, ".zshrc"), []byte("export PATH"), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dir, ".gitconfig"), []byte("[user]"), 0644); err != nil { t.Fatal(err) }

	// Create files to exclude
	if err := os.WriteFile(filepath.Join(dir, "debug.log"), []byte("log data"), 0644); err != nil { t.Fatal(err) }

	// Create Cache directory to exclude
	cacheDir := filepath.Join(dir, "Cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(cacheDir, "data.bin"), []byte("cache"), 0644); err != nil { t.Fatal(err) }

	handler := &DotfilesHandler{}
	cfg := &config.CategoryConfig{
		Enabled: true,
		Paths:   []string{dir + "/"},
		Exclude: []string{"*.log", "Cache"},
	}

	entries, err := handler.Discover(cfg)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	for _, e := range entries {
		base := filepath.Base(e.SourcePath)
		if base == "debug.log" {
			t.Error("debug.log should be excluded")
		}
		if base == "data.bin" {
			t.Error("files inside Cache/ should be excluded")
		}
	}

	// Verify at least the expected files are present
	foundCount := 0
	for _, e := range entries {
		base := filepath.Base(e.SourcePath)
		if base == ".zshrc" || base == ".gitconfig" {
			foundCount++
		}
	}
	if foundCount != 2 {
		t.Errorf("expected 2 kept files, found %d", foundCount)
	}
}

func TestShouldExcludeEntry(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		fileName string
		excludes []string
		want     bool
	}{
		{
			name:     "no excludes",
			relPath:  "file.txt",
			fileName: "file.txt",
			excludes: nil,
			want:     false,
		},
		{
			name:     "exact name match",
			relPath:  "Cache",
			fileName: "Cache",
			excludes: []string{"Cache"},
			want:     true,
		},
		{
			name:     "glob pattern match",
			relPath:  "debug.log",
			fileName: "debug.log",
			excludes: []string{"*.log"},
			want:     true,
		},
		{
			name:     "no match",
			relPath:  "config.yaml",
			fileName: "config.yaml",
			excludes: []string{"*.log", "Cache"},
			want:     false,
		},
		{
			name:     "match in path component",
			relPath:  filepath.Join("Cache", "data.bin"),
			fileName: "data.bin",
			excludes: []string{"Cache"},
			want:     true,
		},
		{
			name:     "nested path no match",
			relPath:  filepath.Join("subdir", "file.txt"),
			fileName: "file.txt",
			excludes: []string{"*.log"},
			want:     false,
		},
		{
			name:     "hidden file pattern",
			relPath:  ".DS_Store",
			fileName: ".DS_Store",
			excludes: []string{".DS_Store"},
			want:     true,
		},
		{
			name:     "wildcard matches hidden files",
			relPath:  ".hidden",
			fileName: ".hidden",
			excludes: []string{".*"},
			want:     true,
		},
		{
			name:     "multiple excludes second matches",
			relPath:  "test.tmp",
			fileName: "test.tmp",
			excludes: []string{"*.log", "*.tmp"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldExcludeEntry(tt.relPath, tt.fileName, tt.excludes)
			if got != tt.want {
				t.Errorf("shouldExcludeEntry(%q, %q, %v) = %v, want %v",
					tt.relPath, tt.fileName, tt.excludes, got, tt.want)
			}
		})
	}
}

func TestDetectAppSource(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name      string
		setup     func() string // returns appPath
		appName   string
		brewCasks map[string]bool
		want      string
	}{
		{
			name: "app store app with receipt",
			setup: func() string {
				appPath := filepath.Join(dir, "Pages.app")
				receiptDir := filepath.Join(appPath, "Contents", "_MASReceipt")
				if err := os.MkdirAll(receiptDir, 0755); err != nil { t.Fatal(err) }
				if err := os.WriteFile(filepath.Join(receiptDir, "receipt"), []byte("receipt-data"), 0644); err != nil { t.Fatal(err) }
				return appPath
			},
			appName:   "Pages",
			brewCasks: map[string]bool{},
			want:      "app-store",
		},
		{
			name: "homebrew cask app",
			setup: func() string {
				appPath := filepath.Join(dir, "Firefox.app")
				if err := os.MkdirAll(appPath, 0755); err != nil { t.Fatal(err) }
				return appPath
			},
			appName:   "Firefox",
			brewCasks: map[string]bool{"firefox": true},
			want:      "homebrew-cask",
		},
		{
			name: "homebrew cask app with spaces",
			setup: func() string {
				appPath := filepath.Join(dir, "Visual Studio Code.app")
				if err := os.MkdirAll(appPath, 0755); err != nil { t.Fatal(err) }
				return appPath
			},
			appName:   "Visual Studio Code",
			brewCasks: map[string]bool{"visual-studio-code": true},
			want:      "homebrew-cask",
		},
		{
			name: "manual app",
			setup: func() string {
				appPath := filepath.Join(dir, "CustomApp.app")
				if err := os.MkdirAll(appPath, 0755); err != nil { t.Fatal(err) }
				return appPath
			},
			appName:   "CustomApp",
			brewCasks: map[string]bool{"firefox": true},
			want:      "manual",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appPath := tt.setup()
			got := detectAppSource(appPath, tt.appName, tt.brewCasks)
			if got != tt.want {
				t.Errorf("detectAppSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectOrigin(t *testing.T) {
	tests := []struct {
		name            string
		binPath         string
		excludePrefixes []string
		want            string
	}{
		{
			name:            "homebrew Cellar path",
			binPath:         "/usr/local/Cellar/git/2.40/bin/git",
			excludePrefixes: []string{"/usr/bin", "/bin"},
			want:            "homebrew",
		},
		{
			name:            "homebrew path",
			binPath:         "/opt/homebrew/bin/git",
			excludePrefixes: []string{"/usr/bin", "/bin"},
			want:            "homebrew",
		},
		{
			name:            "go binary",
			binPath:         "/Users/test/go/bin/golangci-lint",
			excludePrefixes: []string{"/usr/bin", "/bin"},
			want:            "go",
		},
		{
			name:            "cargo binary",
			binPath:         "/Users/test/cargo/bin/rg",
			excludePrefixes: []string{"/usr/bin", "/bin"},
			want:            "cargo",
		},
		{
			name:            "npm binary",
			binPath:         "/Users/test/.npm/bin/eslint",
			excludePrefixes: []string{"/usr/bin", "/bin"},
			want:            "npm",
		},
		{
			name:            "manual binary",
			binPath:         "/usr/local/bin/custom-tool",
			excludePrefixes: []string{"/usr/bin", "/bin"},
			want:            "manual",
		},
		{
			name:            "system binary",
			binPath:         "/usr/bin/ls",
			excludePrefixes: []string{"/usr/bin", "/bin"},
			want:            "system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectOrigin(tt.binPath, tt.excludePrefixes)
			if got != tt.want {
				t.Errorf("detectOrigin(%q) = %q, want %q", tt.binPath, got, tt.want)
			}
		})
	}
}

func TestShouldExcludePref(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		excludes []string
		want     bool
	}{
		{
			name:     "apple plist excluded by wildcard",
			fileName: "com.apple.finder.plist",
			excludes: []string{"com.apple.*"},
			want:     true,
		},
		{
			name:     "non-apple plist not excluded",
			fileName: "com.brave.Browser.plist",
			excludes: []string{"com.apple.*"},
			want:     false,
		},
		{
			name:     "exact match excludes",
			fileName: "com.example.test.plist",
			excludes: []string{"com.example.test.plist"},
			want:     true,
		},
		{
			name:     "no excludes",
			fileName: "com.apple.finder.plist",
			excludes: nil,
			want:     false,
		},
		{
			name:     "empty excludes",
			fileName: "com.apple.finder.plist",
			excludes: []string{},
			want:     false,
		},
		{
			name:     "multiple patterns second matches",
			fileName: "org.mozilla.firefox.plist",
			excludes: []string{"com.apple.*", "org.mozilla.*"},
			want:     true,
		},
		{
			name:     "multiple patterns none match",
			fileName: "com.brave.Browser.plist",
			excludes: []string{"com.apple.*", "org.mozilla.*"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldExcludePref(tt.fileName, tt.excludes)
			if got != tt.want {
				t.Errorf("shouldExcludePref(%q, %v) = %v, want %v",
					tt.fileName, tt.excludes, got, tt.want)
			}
		})
	}
}
