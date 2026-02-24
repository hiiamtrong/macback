package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"tilde only", "~", home},
		{"tilde slash", "~/Documents", filepath.Join(home, "Documents")},
		{"absolute", "/usr/local/bin", "/usr/local/bin"},
		{"relative", "foo/bar", "foo/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandPath(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temp config file
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
backup_dest: /tmp/test-backup
categories:
  ssh:
    enabled: true
    paths:
      - "~/.ssh/config"
    secret_patterns:
      - "id_*"
  shell:
    enabled: true
    paths:
      - "~/.zshrc"
encryption:
  enabled: true
  extension: ".age"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.BackupDest != "/tmp/test-backup" {
		t.Errorf("BackupDest = %q, want %q", cfg.BackupDest, "/tmp/test-backup")
	}

	if len(cfg.Categories) != 2 {
		t.Errorf("Categories count = %d, want 2", len(cfg.Categories))
	}

	ssh := cfg.Categories["ssh"]
	if ssh == nil || !ssh.Enabled {
		t.Error("SSH category should be enabled")
	}
}

func TestLoadConfigNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for missing config")
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// Missing backup_dest
	content := `
categories:
  ssh:
    enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestLoadConfigUnknownCategory(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
backup_dest: /tmp/test
categories:
  unknown_category:
    enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error for unknown category")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.BackupDest == "" {
		t.Error("default BackupDest should not be empty")
	}

	expectedCategories := []string{"ssh", "shell", "git", "dotfiles", "homebrew", "pathbin", "projects"}
	for _, name := range expectedCategories {
		if _, ok := cfg.Categories[name]; !ok {
			t.Errorf("default config missing category %q", name)
		}
	}

	if !cfg.Encryption.Enabled {
		t.Error("encryption should be enabled by default")
	}
}

func TestDefaultProjectsCategory(t *testing.T) {
	cfg := DefaultConfig()

	projects, ok := cfg.Categories["projects"]
	if !ok {
		t.Fatal("default config missing 'projects' category")
	}

	if projects.Enabled {
		t.Error("projects category should be disabled by default")
	}
	if len(projects.ScanDirs) == 0 {
		t.Error("projects category should have default scan_dirs")
	}
	if projects.MaxFileSizeMB != 50 {
		t.Errorf("projects MaxFileSizeMB = %d, want 50", projects.MaxFileSizeMB)
	}
	if projects.ProjectDepth != 1 {
		t.Errorf("projects ProjectDepth = %d, want 1", projects.ProjectDepth)
	}
	if len(projects.Exclude) == 0 {
		t.Error("projects category should have default exclude patterns")
	}
}

func TestValidateAcceptsProjectsCategory(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
backup_dest: /tmp/test
categories:
  projects:
    enabled: false
    scan_dirs:
      - ~/Works
    project_depth: 1
    max_file_size_mb: 50
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() with projects category error: %v", err)
	}

	projects := cfg.Categories["projects"]
	if projects == nil {
		t.Fatal("projects category not loaded")
	}
	if projects.MaxFileSizeMB != 50 {
		t.Errorf("MaxFileSizeMB = %d, want 50", projects.MaxFileSizeMB)
	}
	if projects.ProjectDepth != 1 {
		t.Errorf("ProjectDepth = %d, want 1", projects.ProjectDepth)
	}
}

func TestWriteDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test-config.yaml")

	if err := WriteDefault(cfgPath); err != nil {
		t.Fatalf("WriteDefault() error: %v", err)
	}

	// Verify the file was created and is valid
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() after WriteDefault() error: %v", err)
	}

	if cfg.BackupDest == "" {
		t.Error("loaded config should have backup_dest")
	}
}

func TestEnabledCategories(t *testing.T) {
	cfg := &Config{
		BackupDest: "/tmp/test",
		Categories: map[string]*CategoryConfig{
			"ssh":   {Enabled: true},
			"shell": {Enabled: false},
			"git":   {Enabled: true},
		},
	}

	enabled := cfg.EnabledCategories()
	if len(enabled) != 2 {
		t.Errorf("EnabledCategories() returned %d, want 2", len(enabled))
	}
}

func TestValidateDefaultsExtension(t *testing.T) {
	cfg := &Config{
		BackupDest: "/tmp/test",
		Categories: map[string]*CategoryConfig{},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if cfg.Encryption.Extension != ".age" {
		t.Errorf("Extension = %q, want %q", cfg.Encryption.Extension, ".age")
	}
}
