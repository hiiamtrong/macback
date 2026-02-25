---
phase: implementation
title: "Chrome Profile Backup - Implementation Guide"
description: Technical implementation notes for the browser backup category
---

# Chrome Profile Backup - Implementation Guide

## Development Setup

No new dependencies. Uses only standard library and existing internal packages.

```bash
make build && make test && make lint
```

## Code Structure

### New file
```
internal/backup/browser.go       ← BrowserHandler implementation
internal/backup/browser_test.go  ← Unit + integration tests
```

### Modified files
```
internal/config/config.go        ← add "browser" to validCategories
internal/config/defaults.go      ← add default browser category entry
```

## Implementation Notes

### Known Browsers List

```go
type browserDef struct {
    Name           string // display name used in RelPath
    AppSupportPath string // relative to ~/Library/Application Support/
}

func knownBrowsers() []browserDef {
    return []browserDef{
        {"BraveBrowser",   "BraveSoftware/Brave-Browser"},
        {"GoogleChrome",   "Google/Chrome"},
        {"Chromium",       "Chromium"},
        {"MicrosoftEdge",  "Microsoft Edge"},
        {"Arc",            "Arc/User Data"},
        {"Vivaldi",        "Vivaldi"},
        {"Opera",          "com.operasoftware.Opera"},
        {"OperaGX",        "com.operasoftware.OperaGX"},
    }
}
```

### Built-in Cache Dir Exclusion

```go
var builtinCacheDirs = map[string]bool{
    "Cache":                   true,
    "Code Cache":              true,
    "GPUCache":                true,
    "DawnCache":               true,
    "DawnGraphiteCache":       true,
    "GraphiteDawnCache":       true,
    "GrShaderCache":           true,
    "CacheStorage":            true,
    "ScriptCache":             true,
    "PnaclTranslationCache":   true,
    "ShaderCache":             true,
}

func isBuiltinCacheDir(name string) bool {
    return builtinCacheDirs[name]
}
```

### Profile Discovery

```go
func discoverProfiles(browserDataDir string) []string {
    var profiles []string
    entries, _ := os.ReadDir(browserDataDir)
    for _, e := range entries {
        if !e.IsDir() { continue }
        name := e.Name()
        if name == "Default" || name == "Guest Profile" ||
           strings.HasPrefix(name, "Profile ") {
            profiles = append(profiles, filepath.Join(browserDataDir, name))
        }
    }
    return profiles
}
```

### File Discovery per Profile

```go
func discoverBrowserFiles(browserName, profileDir string, cfg *config.CategoryConfig) []FileEntry {
    var entries []FileEntry
    profileName := filepath.Base(profileDir)
    maxBytes := projectMaxBytes(cfg) // reuse from projects.go

    _ = filepath.WalkDir(profileDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return nil }

        // Built-in cache dirs always excluded
        if d.IsDir() && isBuiltinCacheDir(d.Name()) {
            return filepath.SkipDir
        }

        relToProfile, _ := filepath.Rel(profileDir, path)

        // User-defined excludes
        if path != profileDir && shouldExcludeEntry(relToProfile, d.Name(), cfg.Exclude) {
            if d.IsDir() { return filepath.SkipDir }
            return nil
        }

        if d.IsDir() { return nil }

        entry := buildFileEntry(
            filepath.Dir(profileDir), // scanDir = browser data root
            path, d, maxBytes,
        )
        if entry == nil { return nil }

        // Override RelPath to include browser name + profile name
        relToProfile, _ = filepath.Rel(profileDir, path)
        entry.RelPath = filepath.Join("browser", browserName, profileName, relToProfile)
        entry.Category = "browser"
        entries = append(entries, *entry)
        return nil
    })
    return entries
}
```

> **Note**: Reuses `buildFileEntry()` and `projectMaxBytes()` from `projects.go` (same package).

### Discover Method

```go
func (h *BrowserHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
    var entries []FileEntry
    appSupport := filepath.Join(os.Getenv("HOME"), "Library", "Application Support")

    // Auto-detect known browsers
    for _, b := range knownBrowsers() {
        browserDir := filepath.Join(appSupport, b.AppSupportPath)
        if !fsutil.DirExists(browserDir) { continue }
        for _, profile := range discoverProfiles(browserDir) {
            entries = append(entries, discoverBrowserFiles(b.Name, profile, cfg)...)
        }
    }

    // Custom paths via ScanDirs
    for _, customDir := range cfg.ScanDirs {
        expanded, err := fsutil.ExpandPath(customDir)
        if err != nil || !fsutil.DirExists(expanded) { continue }
        name := filepath.Base(expanded)
        for _, profile := range discoverProfiles(expanded) {
            entries = append(entries, discoverBrowserFiles(name, profile, cfg)...)
        }
    }

    return entries, nil
}
```

### Backup Method

Identical pattern to `dotfiles.go` — copy or encrypt per entry:

```go
func (h *BrowserHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
    result := &CategoryResult{CategoryName: "browser"}
    for _, entry := range entries {
        select {
        case <-ctx.Done(): return nil, ctx.Err()
        default:
        }
        dstPath := filepath.Join(dest, entry.RelPath)
        hash, err := fsutil.SHA256File(entry.SourcePath)
        if err != nil {
            result.Warnings = append(result.Warnings, fmt.Sprintf("skipping %s: %v", entry.RelPath, err))
            continue
        }
        me := &ManifestEntry{
            Original: fsutil.ContractPath(entry.SourcePath),
            Size:     entry.Size, Mode: fsutil.FileModeString(entry.Mode),
            ModTime: entry.ModTime, SHA256: hash,
        }
        if entry.IsSecret {
            encPath, err := enc.EncryptFile(entry.SourcePath, dstPath)
            if err != nil {
                result.Warnings = append(result.Warnings, fmt.Sprintf("encrypting %s: %v", entry.RelPath, err))
                continue
            }
            me.Path = filepath.Join("browser", entry.RelPath+filepath.Ext(encPath))
            me.Encrypted = true
        } else {
            if err := fsutil.CopyFile(entry.SourcePath, dstPath); err != nil {
                result.Warnings = append(result.Warnings, fmt.Sprintf("copying %s: %v", entry.RelPath, err))
                continue
            }
            me.Path = filepath.Join("browser", entry.RelPath)
        }
        result.Entries = append(result.Entries, *me)
        result.FileCount++
        if me.Encrypted { result.EncryptedCount++ }
    }
    return result, nil
}
```

## Integration Points

- **Engine**: No changes. `BrowserHandler` registered via `init()`.
- **`shouldExcludeEntry`**: Reused from `dotfiles.go` (same package).
- **`buildFileEntry` / `projectMaxBytes`**: Reused from `projects.go` (same package).
- **Restore engine**: No changes — restores by `RelPath`.

## Error Handling

- Browser dir not found → skip silently (browser not installed)
- Profile dir read error → skip with warning
- File copy error (browser locked) → warn per file, continue
- Context cancel → propagated

## Security Notes

- `Login Data` and `Cookies` are macOS-Keychain-encrypted blobs — safe to copy, do NOT re-encrypt with age (would corrupt them)
- `Preferences` and `Secure Preferences` contain no raw secrets — safe to copy plaintext
- If user wants history excluded for privacy: add `History` to `exclude` in config
