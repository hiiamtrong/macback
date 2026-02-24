---
phase: implementation
title: "Project Backup - Implementation Guide"
description: Technical implementation notes for the projects backup category
---

# Project Backup - Implementation Guide

## Development Setup

No new dependencies required. The feature uses only the Go standard library and existing internal packages.

```bash
# Build and test as usual
make build
make test
make lint
```

## Code Structure

### New file
```
internal/backup/projects.go       ← ProjectsHandler implementation
internal/backup/projects_test.go  ← Unit + integration tests
```

### Modified files
```
internal/config/config.go         ← Add MaxFileSizeMB, ProjectDepth to CategoryConfig; add "projects" to validCategories
internal/config/defaults.go       ← Add default projects category entry
```

## Implementation Notes

### Core Discovery Logic

```go
// internal/backup/projects.go

func init() { Register(&ProjectsHandler{}) }

type ProjectsHandler struct{}

func (h *ProjectsHandler) Name() string { return "projects" }

func (h *ProjectsHandler) Discover(cfg *config.CategoryConfig) ([]FileEntry, error) {
    var entries []FileEntry
    depth := cfg.ProjectDepth
    if depth <= 0 { depth = 1 }

    for _, scanDir := range cfg.ScanDirs {
        expanded, err := fsutil.ExpandPath(scanDir)
        if err != nil || !fsutil.DirExists(expanded) {
            // add to warnings — caller collects via CategoryResult
            continue
        }
        projectRoots, _ := collectProjectRoots(expanded, depth)
        for _, root := range projectRoots {
            fileEntries := discoverProjectFiles(expanded, root, cfg)
            entries = append(entries, fileEntries...)
        }
    }
    return entries, nil
}

// collectProjectRoots returns directories at exactly `depth` levels below base.
func collectProjectRoots(base string, depth int) ([]string, error) {
    var roots []string
    _ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
        if err != nil || path == base { return nil }
        rel, _ := filepath.Rel(base, path)
        parts := strings.Split(rel, string(filepath.Separator))
        if len(parts) == depth && d.IsDir() {
            roots = append(roots, path)
            return filepath.SkipDir
        }
        if len(parts) > depth { return filepath.SkipDir }
        return nil
    })
    return roots, nil
}

// discoverProjectFiles walks a single project directory.
func discoverProjectFiles(scanDir, projectDir string, cfg *config.CategoryConfig) []FileEntry {
    var entries []FileEntry
    maxBytes := int64(cfg.MaxFileSizeMB) * 1024 * 1024  // 0 = unlimited

    _ = filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return nil }
        relToProject, _ := filepath.Rel(projectDir, path)
        if shouldExcludeEntry(relToProject, d.Name(), cfg.Exclude) {
            if d.IsDir() { return filepath.SkipDir }
            return nil
        }
        if d.IsDir() { return nil }
        info, err := d.Info()
        if err != nil { return nil }
        if info.Mode()&os.ModeSymlink != 0 { return nil }
        if maxBytes > 0 && info.Size() > maxBytes { return nil }

        relPath, _ := filepath.Rel(scanDir, path)
        entries = append(entries, FileEntry{
            SourcePath: path,
            RelPath:    relPath,
            Category:   "projects",
            Mode:       info.Mode().Perm(),
            ModTime:    info.ModTime(),
            Size:       info.Size(),
        })
        return nil
    })
    return entries
}
```

### Backup Method

Reuse `BackupFileEntry` (from `engine.go`) per file — same pattern as SSH handler:

```go
func (h *ProjectsHandler) Backup(ctx context.Context, entries []FileEntry, dest string, enc crypto.Encryptor) (*CategoryResult, error) {
    result := &CategoryResult{CategoryName: "projects"}
    for _, entry := range entries {
        select {
        case <-ctx.Done(): return nil, ctx.Err()
        default:
        }
        me, err := BackupFileEntry(entry, filepath.Join(dest, filepath.Dir(entry.RelPath)), enc)
        if err != nil {
            result.Warnings = append(result.Warnings, fmt.Sprintf("skipping %s: %v", entry.RelPath, err))
            continue
        }
        result.Entries = append(result.Entries, *me)
        result.FileCount++
        if me.Encrypted { result.EncryptedCount++ }
    }
    return result, nil
}
```

> **Note**: `BackupFileEntry` expects `dest` to be the directory for the file. Pass `filepath.Join(dest, filepath.Dir(entry.RelPath))` so nested paths are preserved.

### Config Changes

```go
// internal/config/config.go — add to CategoryConfig struct:
MaxFileSizeMB int `yaml:"max_file_size_mb,omitempty"`
ProjectDepth  int `yaml:"project_depth,omitempty"`

// internal/config/config.go — add to validCategories:
"projects": true,
```

### Default Config Entry

```go
// internal/config/defaults.go — add to DefaultConfig():
"projects": {
    Enabled:      false,
    ScanDirs:     []string{"~/Works", "~/projects"},
    ProjectDepth: 1,
    MaxFileSizeMB: 50,
    Exclude: []string{
        // Package managers
        "node_modules", "vendor", ".venv", "venv", "env", "Pods",
        // Build outputs
        "__pycache__", ".gradle", "build", "target", "dist", ".next", ".nuxt", "out",
        // Caches & VCS
        ".cache", ".git", ".DS_Store",
        // Binary artifacts
        "*.pyc", "*.class", "*.o", "*.a",
    },
    SecretPatterns: []string{".env", ".env.*", "*secret*", "*.pem", "*.key"},
},
```

## Integration Points

- **Engine**: No changes. `ProjectsHandler` is registered via `init()` and the engine picks it up automatically.
- **Restore engine**: No changes. Restores files by their `RelPath` just like any other category.
- **Config validation**: `"projects"` added to whitelist — existing validation logic works as-is.

## Error Handling

- Missing `scan_dir`: warn in `CategoryResult.Warnings`, continue to next dir
- File copy error: warn, skip file, continue
- File too large: silently skip (or warn if `verbose` — use `e.log.Verbose` level)
- Ctx cancellation: propagated via `select` in Backup loop

## Performance Considerations

- `filepath.WalkDir` is O(all files). For 50 projects × 10k files = 500k `stat` calls — typically completes in <5s.
- `SkipDir` on excluded dirs (node_modules etc.) prunes the largest subtrees early — this is the most important optimization.
- Incremental backup in the engine (SHA256 compare + copy-from-rotated) handles re-runs efficiently.
- `max_file_size_mb` prevents accidentally copying large binary assets.

## Security Notes

- `.env` and other secret files within projects are encrypted by the existing `secret_patterns` + `IsSecret` mechanism — no special handling needed.
- No new attack surface introduced (no shell execution, no network calls).
