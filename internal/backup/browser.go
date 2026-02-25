package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiiamtrong/macback/internal/config"
	"github.com/hiiamtrong/macback/internal/crypto"
	"github.com/hiiamtrong/macback/internal/fsutil"
)

func init() {
	Register(&BrowserHandler{})
}

// BrowserHandler backs up Chromium-based browser profiles (bookmarks,
// extensions, preferences, etc.) while excluding cache directories.
type BrowserHandler struct{}

func (h *BrowserHandler) Name() string { return "browser" }

// browserDef describes a known Chromium-based browser.
type browserDef struct {
	Name           string // filesystem-safe name used in RelPath
	AppSupportPath string // path relative to ~/Library/Application Support/
}

// knownBrowsers returns the list of supported Chromium-based browsers.
func knownBrowsers() []browserDef {
	return []browserDef{
		{"BraveBrowser", "BraveSoftware/Brave-Browser"},
		{"GoogleChrome", "Google/Chrome"},
		{"Chromium", "Chromium"},
		{"MicrosoftEdge", "Microsoft Edge"},
		{"Arc", "Arc/User Data"},
		{"Vivaldi", "Vivaldi"},
		{"Opera", "com.operasoftware.Opera"},
		{"OperaGX", "com.operasoftware.OperaGX"},
	}
}

// builtinCacheDirs is the set of directory names that are always excluded
// regardless of user configuration.
var builtinCacheDirs = map[string]bool{
	// GPU / shader caches
	"Cache":                 true,
	"Code Cache":            true,
	"GPUCache":              true,
	"DawnCache":             true,
	"DawnGraphiteCache":     true,
	"DawnWebGPUCache":       true,
	"GraphiteDawnCache":     true,
	"GrShaderCache":         true,
	"CacheStorage":          true,
	"ScriptCache":           true,
	"PnaclTranslationCache": true,
	"ShaderCache":           true,
	// Large ephemeral storage — rebuilt automatically by browser/web apps
	"IndexedDB":         true,
	"File System":       true,
	"Shared Dictionary": true,
}

// isBuiltinCacheDir reports whether name is a well-known Chromium cache directory.
func isBuiltinCacheDir(name string) bool {
	return builtinCacheDirs[name]
}

// discoverProfiles returns profile subdirectories (Default, Profile N, Guest Profile)
// found directly under browserDataDir.
func discoverProfiles(browserDataDir string) []string {
	var profiles []string

	entries, err := os.ReadDir(browserDataDir)
	if err != nil {
		return profiles
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "Default" || name == "Guest Profile" || strings.HasPrefix(name, "Profile ") {
			profiles = append(profiles, filepath.Join(browserDataDir, name))
		}
	}

	return profiles
}

// buildBrowserEntry creates a FileEntry for a browser profile file, or returns nil
// if the file should be skipped (unreadable, symlink, or over the size limit).
func buildBrowserEntry(browserName, profileName, profileDir, path string, d fs.DirEntry, maxBytes int64) *FileEntry {
	info, err := d.Info()
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return nil
	}
	relToProfile, _ := filepath.Rel(profileDir, path)
	return &FileEntry{
		SourcePath: path,
		RelPath:    filepath.Join(browserName, profileName, relToProfile),
		Category:   "browser",
		Mode:       info.Mode().Perm(),
		ModTime:    info.ModTime(),
		Size:       info.Size(),
	}
}

// browserRootFiles lists root-level browser data files that must be backed up
// for a complete restore (e.g. Local State contains the profile registry).
var browserRootFiles = []string{"Local State"}

// discoverBrowserRootFiles returns FileEntries for important root-level browser
// data files (outside profile directories) such as "Local State".
func discoverBrowserRootFiles(browserName, browserDir string) []FileEntry {
	var entries []FileEntry
	for _, name := range browserRootFiles {
		path := filepath.Join(browserDir, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		entries = append(entries, FileEntry{
			SourcePath: path,
			RelPath:    filepath.Join(browserName, name),
			Category:   "browser",
			Mode:       info.Mode().Perm(),
			ModTime:    info.ModTime(),
			Size:       info.Size(),
		})
	}
	return entries
}

// discoverBrowserFiles walks a single browser profile directory and collects
// eligible files, applying cache exclusions, user excludes, and size filter.
func discoverBrowserFiles(browserName, profileDir string, cfg *config.CategoryConfig) []FileEntry {
	var entries []FileEntry
	profileName := filepath.Base(profileDir)
	maxBytes := projectMaxBytes(cfg)

	_ = filepath.WalkDir(profileDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && isBuiltinCacheDir(d.Name()) {
			return filepath.SkipDir
		}
		relToProfile, _ := filepath.Rel(profileDir, path)
		if path != profileDir && shouldExcludeEntry(relToProfile, d.Name(), cfg.Exclude) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if entry := buildBrowserEntry(browserName, profileName, profileDir, path, d, maxBytes); entry != nil {
			entries = append(entries, *entry)
		}
		return nil
	})

	return entries
}

// Discover finds all browser profile files across all known (and custom) browsers.
func (h *BrowserHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
	var entries []FileEntry

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}
	appSupport := filepath.Join(home, "Library", "Application Support")

	// Auto-detect known browsers
	for _, b := range knownBrowsers() {
		browserDir := filepath.Join(appSupport, b.AppSupportPath)
		if !fsutil.DirExists(browserDir) {
			continue
		}
		entries = append(entries, discoverBrowserRootFiles(b.Name, browserDir)...)
		for _, profile := range discoverProfiles(browserDir) {
			entries = append(entries, discoverBrowserFiles(b.Name, profile, cfg)...)
		}
	}

	// Custom browser paths via ScanDirs
	for _, customDir := range cfg.ScanDirs {
		expanded, err := fsutil.ExpandPath(customDir)
		if err != nil || !fsutil.DirExists(expanded) {
			continue
		}
		name := filepath.Base(expanded)
		entries = append(entries, discoverBrowserRootFiles(name, expanded)...)
		for _, profile := range discoverProfiles(expanded) {
			entries = append(entries, discoverBrowserFiles(name, profile, cfg)...)
		}
	}

	return entries, nil
}

// patchLocalStateContent reads a Chrome "Local State" JSON file and injects any
// profile directories found in browserDir that are not yet in info_cache.
// This ensures that all backed-up profiles are registered after a restore.
func patchLocalStateContent(localStatePath, browserDir string) ([]byte, error) {
	raw, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, err
	}

	var state map[string]json.RawMessage
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}

	// Navigate profile.info_cache
	var profileSection map[string]json.RawMessage
	if v, ok := state["profile"]; ok {
		_ = json.Unmarshal(v, &profileSection)
	}
	if profileSection == nil {
		profileSection = make(map[string]json.RawMessage)
	}

	var infoCache map[string]json.RawMessage
	if v, ok := profileSection["info_cache"]; ok {
		_ = json.Unmarshal(v, &infoCache)
	}
	if infoCache == nil {
		infoCache = make(map[string]json.RawMessage)
	}

	// Scan for profile directories not yet registered
	dirEntries, err := os.ReadDir(browserDir)
	if err != nil {
		return raw, nil // return original on read error
	}

	added := 0
	for _, e := range dirEntries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name != "Default" && name != "Guest Profile" && !strings.HasPrefix(name, "Profile ") {
			continue
		}
		if _, exists := infoCache[name]; exists {
			continue
		}

		// Try to get the human-readable profile name from Preferences
		profileName := name
		prefsPath := filepath.Join(browserDir, name, "Preferences")
		if prefsData, err := os.ReadFile(prefsPath); err == nil {
			var prefs struct {
				Profile struct {
					Name string `json:"name"`
				} `json:"profile"`
			}
			if json.Unmarshal(prefsData, &prefs) == nil && prefs.Profile.Name != "" {
				profileName = prefs.Profile.Name
			}
		}

		entry := map[string]interface{}{
			"name":                  profileName,
			"is_using_default_name": false,
			"active_time":           0.0,
		}
		entryJSON, _ := json.Marshal(entry)
		infoCache[name] = entryJSON
		added++
	}

	if added == 0 {
		return raw, nil // nothing changed
	}

	// Rebuild profile section
	infoCacheJSON, err := json.Marshal(infoCache)
	if err != nil {
		return raw, nil
	}
	profileSection["info_cache"] = infoCacheJSON
	profileSectionJSON, err := json.Marshal(profileSection)
	if err != nil {
		return raw, nil
	}
	state["profile"] = profileSectionJSON

	return json.Marshal(state)
}

// sha256HexBytes returns the hex-encoded SHA-256 hash of data.
func sha256HexBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Backup copies each browser profile file to the destination directory.
func (h *BrowserHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
	result := &CategoryResult{
		CategoryName: "browser",
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		dstPath := filepath.Join(dest, entry.RelPath)

		hash, err := fsutil.SHA256File(entry.SourcePath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skipping %s: %v", entry.RelPath, err))
			continue
		}

		me := &ManifestEntry{
			Original:  fsutil.ContractPath(entry.SourcePath),
			Size:      entry.Size,
			Mode:      fsutil.FileModeString(entry.Mode),
			ModTime:   entry.ModTime,
			SHA256:    hash,
			Encrypted: false,
		}

		if entry.IsSecret {
			encPath, err := enc.EncryptFile(entry.SourcePath, dstPath)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("encrypting %s: %v", entry.RelPath, err))
				continue
			}
			me.Path = filepath.Join("browser", entry.RelPath) + filepath.Ext(encPath)
			me.Encrypted = true
		} else if filepath.Base(entry.RelPath) == "Local State" {
			// Patch Local State to include all discovered profile dirs before saving.
			browserDir := filepath.Dir(entry.SourcePath)
			patched, err := patchLocalStateContent(entry.SourcePath, browserDir)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("patching Local State %s: %v", entry.RelPath, err))
				continue
			}
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("creating dir for %s: %v", entry.RelPath, err))
				continue
			}
			if err := os.WriteFile(dstPath, patched, entry.Mode); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("writing Local State %s: %v", entry.RelPath, err))
				continue
			}
			me.SHA256 = sha256HexBytes(patched)
			me.Path = filepath.Join("browser", entry.RelPath)
		} else {
			if err := fsutil.CopyFile(entry.SourcePath, dstPath); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("copying %s: %v", entry.RelPath, err))
				continue
			}
			me.Path = filepath.Join("browser", entry.RelPath)
		}

		result.Entries = append(result.Entries, *me)
		result.FileCount++
		if me.Encrypted {
			result.EncryptedCount++
		}
	}

	return result, nil
}
