---
phase: planning
title: Backup ZIP — Task Breakdown
description: Implementation tasks for zip compression feature
---

# Project Planning & Task Breakdown

## Milestones

- [ ] M1: Core zip utilities (`ZipDir` + `UnzipToTemp`) with full tests
- [ ] M2: Backup CLI zip support (`--zip`, `--zip-only`, config field)
- [ ] M3: Restore/list/diff zip-source support
- [ ] M4: Rotation cleanup includes `.zip` companions

---

## Task Breakdown

### Phase 1: Core Zip Utilities

- [ ] **T1.1** Create `internal/fsutil/zip.go`
  - `ZipDir(srcDir, destZip string) error`
    - Walk srcDir, write each file to zip with relative path
    - Write to temp file first, rename on success (atomic)
    - Skip symlinks
    - Use `flate.DefaultCompression`
  - `UnzipToTemp(zipPath string) (dir string, cleanup func(), err error)`
    - Create `os.MkdirTemp`
    - Iterate zip entries; reject paths containing `..` (zip-slip guard)
    - Recreate directory structure; extract files

- [ ] **T1.2** Write `internal/fsutil/zip_test.go`
  - `TestZipDir_RoundTrip` — zip a temp dir, unzip, verify all files match
  - `TestZipDir_AtomicWrite` — verify no partial zip left on error
  - `TestUnzipToTemp_ZipSlipRejected` — entry with `../evil` must error
  - `TestUnzipToTemp_NestedDirs` — verify nested dirs recreated correctly
  - `TestUnzipToTemp_CleanupRemovesDir` — cleanup() deletes temp dir

### Phase 2: Config + Backup CLI

- [ ] **T2.1** Add config fields to `internal/config/config.go`
  - `Zip     bool \`yaml:"zip"\``
  - `ZipOnly bool \`yaml:"zip_only"\``
  - Update `defaults.go` (default: both false)

- [ ] **T2.2** Add `--zip` and `--zip-only` flags to `internal/cli/backup.go`
  - After successful `Engine.Run()`: if `--zip` or `cfg.Zip` → call `fsutil.ZipDir`
  - If `--zip-only` or `cfg.ZipOnly` → remove uncompressed dir after zipping
  - Log zip output path on success; warn on failure (non-fatal)

- [ ] **T2.3** Update `init` command default config template in `internal/cli/init.go`
  - Add `zip: false` and `zip_only: false` to generated config

- [ ] **T2.4** Update README with `--zip` / `--zip-only` docs and config example

### Phase 3: Restore / List / Diff ZIP source

- [ ] **T3.1** Add `resolveBackupSource` helper in `internal/cli/` (shared)
  ```go
  func resolveBackupSource(src string) (dir string, cleanup func(), err error)
  ```
  - If `strings.HasSuffix(src, ".zip")` → `fsutil.UnzipToTemp(src)`
  - Otherwise → `(src, func(){}, nil)`

- [ ] **T3.2** Update `internal/cli/restore.go` to use `resolveBackupSource`

- [ ] **T3.3** Update `internal/cli/list.go` to use `resolveBackupSource`

- [ ] **T3.4** Update `internal/cli/diff.go` to use `resolveBackupSource`

### Phase 4: Rotation Cleanup

- [ ] **T4.1** Extend `cleanOldBackups` in `internal/backup/engine.go`
  - When removing a rotated dir `<dir>`, also remove `<dir>.zip` if it exists

---

## Dependencies

```
T1.1 → T1.2 (tests depend on impl)
T1.1 → T2.2, T3.1 (CLI depends on fsutil)
T2.1 → T2.2 (flags read config fields)
T3.1 → T3.2, T3.3, T3.4 (all commands share helper)
T2.2 → T4.1 (rotation is part of engine)
```

## Risks & Mitigation

| Risk | Likelihood | Mitigation |
|------|-----------|-----------|
| Large backups slow to zip | Medium | Stream files; use DefaultCompression; user can skip with no `--zip` |
| Zip-slip path traversal | Low | Explicit `..` check in `UnzipToTemp`; test coverage |
| Partial zip on failure | Low | Atomic write (temp + rename) |
| `--zip-only` loses incremental | Medium | Document clearly; warn in CLI if `--zip-only` used |
| Old `.zip` not cleaned by rotation | Low | T4.1 handles this |
