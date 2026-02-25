---
phase: implementation
title: Backup ZIP — Implementation Guide
description: Key implementation notes and code patterns for the zip feature
---

# Implementation Guide

## Development Setup

No new dependencies — uses Go stdlib `archive/zip` and `compress/flate` only.

```bash
go test ./internal/fsutil/... # run zip utility tests
go test ./...                  # full suite
make lint
```

## Code Structure

```
internal/
  fsutil/
    zip.go          # NEW: ZipDir + UnzipToTemp
    zip_test.go     # NEW: round-trip, zip-slip, atomic write tests
  config/
    config.go       # ADD: Zip bool, ZipOnly bool fields
    defaults.go     # ADD: zip: false, zip_only: false
  cli/
    backup.go       # ADD: --zip / --zip-only flags; call ZipDir after Engine.Run
    restore.go      # MOD: resolveBackupSource
    list.go         # MOD: resolveBackupSource
    diff.go         # MOD: resolveBackupSource
    zip_source.go   # NEW: resolveBackupSource helper
  backup/
    engine.go       # MOD: cleanOldBackups removes .zip companions
```

## Implementation Notes

### `ZipDir` — atomic write pattern

```go
func ZipDir(srcDir, destZip string) error {
    // Write to temp file first to avoid partial archives
    tmp, err := os.CreateTemp(filepath.Dir(destZip), ".macback-zip-*")
    if err != nil { return err }
    tmpPath := tmp.Name()
    defer func() { _ = os.Remove(tmpPath) }() // remove temp on any error path

    zw := zip.NewWriter(tmp)
    err = filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() { return err }
        if d.Type()&fs.ModeSymlink != 0 { return nil } // skip symlinks
        rel, _ := filepath.Rel(srcDir, path)
        w, err := zw.CreateHeader(&zip.FileHeader{
            Name:   rel,
            Method: zip.Deflate,
        })
        if err != nil { return err }
        f, err := os.Open(path)
        if err != nil { return err }
        defer f.Close()
        _, err = io.Copy(w, f)
        return err
    })
    if err != nil { _ = zw.Close(); return err }
    if err = zw.Close(); err != nil { return err }
    if err = tmp.Close(); err != nil { return err }
    return os.Rename(tmpPath, destZip) // atomic rename
}
```

### `UnzipToTemp` — zip-slip guard

```go
func UnzipToTemp(zipPath string) (string, func(), error) {
    dir, err := os.MkdirTemp("", "macback-restore-*")
    if err != nil { return "", nil, err }
    cleanup := func() { _ = os.RemoveAll(dir) }

    r, err := zip.OpenReader(zipPath)
    if err != nil { cleanup(); return "", nil, err }
    defer r.Close()

    for _, f := range r.File {
        // Zip-slip guard: reject any path containing ".."
        if strings.Contains(filepath.ToSlash(f.Name), "..") {
            cleanup()
            return "", nil, fmt.Errorf("zip entry %q contains unsafe path", f.Name)
        }
        dest := filepath.Join(dir, filepath.FromSlash(f.Name))
        if f.FileInfo().IsDir() {
            _ = os.MkdirAll(dest, 0755)
            continue
        }
        if err := extractZipFile(f, dest); err != nil {
            cleanup()
            return "", nil, err
        }
    }
    return dir, cleanup, nil
}
```

### `resolveBackupSource` — shared CLI helper

```go
// internal/cli/zip_source.go
func resolveBackupSource(src string) (dir string, cleanup func(), err error) {
    if strings.HasSuffix(strings.ToLower(src), ".zip") {
        expanded, err := fsutil.ExpandPath(src)
        if err != nil { return "", nil, err }
        return fsutil.UnzipToTemp(expanded)
    }
    expanded, err := fsutil.ExpandPath(src)
    if err != nil { return "", nil, err }
    return expanded, func() {}, nil
}
```

### Backup CLI — zip after engine run

```go
// cli/backup.go — after engine.Run() succeeds
doZip  := zipFlag || cfg.Zip
doOnly := zipOnlyFlag || cfg.ZipOnly
if doZip {
    destZip := dest + ".zip"
    log.Info("Compressing backup...")
    if err := fsutil.ZipDir(dest, destZip); err != nil {
        log.Warn("zip failed: %v", err)
    } else {
        log.Info("Compressed: %s", fsutil.ContractPath(destZip))
        if doOnly {
            if err := os.RemoveAll(dest); err != nil {
                log.Warn("removing uncompressed dir: %v", err)
            }
        }
    }
}
```

### Rotation cleanup — remove `.zip` companion

```go
// engine.go cleanOldBackups — inside removal loop
for _, old := range toRemove {
    _ = os.RemoveAll(old)
    _ = os.Remove(old + ".zip") // remove companion zip if present
}
```

## Error Handling

- `ZipDir` failure is **non-fatal** in backup: log warning, backup dir remains usable
- `UnzipToTemp` failure is **fatal** for restore/list/diff: return error to user
- Partial zip: temp file removed by defer; no `.zip` artifact left on failure

## Security Notes

- **Zip-slip**: `UnzipToTemp` rejects any entry path containing `..`
- No execution of zip contents; only `io.Copy` to files
- Extracted temp dir uses `0600`/`0700` permissions on sensitive files (same as restore engine)
