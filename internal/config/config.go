package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	ErrConfigNotFound = errors.New("config file not found")
	ErrInvalidConfig  = errors.New("invalid configuration")
)

// Config is the top-level configuration loaded from YAML.
type Config struct {
	BackupDest string                     `yaml:"backup_dest"`
	MaxBackups int                        `yaml:"max_backups"`
	Categories map[string]*CategoryConfig `yaml:"categories"`
	Encryption EncryptionConfig           `yaml:"encryption"`
}

// CategoryConfig defines settings for a single backup category.
type CategoryConfig struct {
	Enabled         bool     `yaml:"enabled"`
	Paths           []string `yaml:"paths,omitempty"`
	SecretPatterns  []string `yaml:"secret_patterns,omitempty"`
	Exclude         []string `yaml:"exclude,omitempty"`
	IncludeCasks    bool     `yaml:"include_casks,omitempty"`
	IncludeTaps     bool     `yaml:"include_taps,omitempty"`
	IncludeMas      bool     `yaml:"include_mas,omitempty"`
	ScanDirs        []string `yaml:"scan_dirs,omitempty"`
	CatalogOnly     bool     `yaml:"catalog_only,omitempty"`
	ExcludePrefixes []string `yaml:"exclude_prefixes,omitempty"`
}

// EncryptionConfig defines global encryption settings.
type EncryptionConfig struct {
	Enabled              bool     `yaml:"enabled"`
	GlobalSecretPatterns []string `yaml:"global_secret_patterns"`
	Extension            string   `yaml:"extension"`
}

// Load reads and validates a config file from the given path.
func Load(path string) (*Config, error) {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return nil, fmt.Errorf("expanding config path: %w", err)
	}

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s (run 'macback init' to create one)", ErrConfigNotFound, expandedPath)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks required fields and applies defaults.
func (c *Config) Validate() error {
	if c.BackupDest == "" {
		return fmt.Errorf("%w: backup_dest is required", ErrInvalidConfig)
	}

	if c.MaxBackups <= 0 {
		c.MaxBackups = 3
	}

	if c.Encryption.Extension == "" {
		c.Encryption.Extension = ".age"
	}

	if c.Categories == nil {
		c.Categories = make(map[string]*CategoryConfig)
	}

	validCategories := map[string]bool{
		"ssh": true, "shell": true, "git": true,
		"dotfiles": true, "homebrew": true, "pathbin": true,
		"mas": true, "appsettings": true, "apps": true,
	}

	for name := range c.Categories {
		if !validCategories[name] {
			return fmt.Errorf("%w: unknown category %q", ErrInvalidConfig, name)
		}
	}

	return nil
}

// EnabledCategories returns the names of all enabled categories.
func (c *Config) EnabledCategories() []string {
	var names []string
	for name, cat := range c.Categories {
		if cat.Enabled {
			names = append(names, name)
		}
	}
	return names
}

// ExpandPath expands ~ to home directory in a path string.
func ExpandPath(p string) (string, error) {
	if p == "" {
		return p, nil
	}

	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		return home, nil
	}

	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		return filepath.Join(home, p[2:]), nil
	}

	return p, nil
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".macback.yaml"
	}
	return filepath.Join(home, ".macback.yaml")
}
