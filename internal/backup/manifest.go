package backup

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Manifest represents the backup manifest file.
type Manifest struct {
	Version        int                          `yaml:"version"`
	CreatedAt      time.Time                    `yaml:"created_at"`
	Hostname       string                       `yaml:"hostname"`
	Username       string                       `yaml:"username"`
	MacOSVersion   string                       `yaml:"macos_version"`
	MacbackVersion string                       `yaml:"macback_version"`
	Categories     map[string]*ManifestCategory `yaml:"categories"`
}

// ManifestCategory holds backup info for one category.
type ManifestCategory struct {
	BackedUp       bool            `yaml:"backed_up"`
	FileCount      int             `yaml:"file_count"`
	EncryptedCount int             `yaml:"encrypted_count"`
	Files          []ManifestEntry `yaml:"files"`
}

// ManifestEntry represents a single file in the backup manifest.
type ManifestEntry struct {
	Path      string    `yaml:"path"`
	Original  string    `yaml:"original"`
	Size      int64     `yaml:"size"`
	Mode      string    `yaml:"mode"`
	ModTime   time.Time `yaml:"mod_time"`
	SHA256    string    `yaml:"sha256"`
	Encrypted bool      `yaml:"encrypted"`
}

// NewManifest creates a new manifest with system info.
func NewManifest(version string) *Manifest {
	hostname, _ := os.Hostname()
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	return &Manifest{
		Version:        1,
		CreatedAt:      time.Now(),
		Hostname:       hostname,
		Username:       username,
		MacOSVersion:   getMacOSVersion(),
		MacbackVersion: version,
		Categories:     make(map[string]*ManifestCategory),
	}
}

// WriteManifest writes the manifest to the backup destination.
func WriteManifest(m *Manifest, dest string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	path := filepath.Join(dest, "manifest.yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

// ReadManifest reads a manifest from a backup folder.
func ReadManifest(source string) (*Manifest, error) {
	path := filepath.Join(source, "manifest.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	return &m, nil
}

// TotalFiles returns the total number of backed-up files across all categories.
func (m *Manifest) TotalFiles() int {
	total := 0
	for _, cat := range m.Categories {
		total += cat.FileCount
	}
	return total
}

// TotalEncrypted returns the total number of encrypted files.
func (m *Manifest) TotalEncrypted() int {
	total := 0
	for _, cat := range m.Categories {
		total += cat.EncryptedCount
	}
	return total
}

// HasEncryptedFiles returns true if any files in the manifest are encrypted.
func (m *Manifest) HasEncryptedFiles() bool {
	return m.TotalEncrypted() > 0
}

// getMacOSVersion returns the macOS version string.
func getMacOSVersion() string {
	data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist")
	if err != nil {
		return "unknown"
	}
	// Simple extraction - look for ProductVersion
	s := string(data)
	key := "<key>ProductVersion</key>"
	idx := 0
	for i := 0; i < len(s)-len(key); i++ {
		if s[i:i+len(key)] == key {
			idx = i + len(key)
			break
		}
	}
	if idx == 0 {
		return "unknown"
	}
	// Find the next <string>...</string>
	start := 0
	end := 0
	for i := idx; i < len(s)-8; i++ {
		if s[i:i+8] == "<string>" {
			start = i + 8
		}
		if s[i:i+9] == "</string>" && start > 0 {
			end = i
			break
		}
	}
	if start > 0 && end > start {
		return s[start:end]
	}
	return "unknown"
}
