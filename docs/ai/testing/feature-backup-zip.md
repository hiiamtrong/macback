---
phase: testing
title: Backup ZIP — Testing Strategy
description: Test cases for zip compression and zip-aware restore
---

# Testing Strategy

## Test Coverage Goals

- Unit tests: 100% of `fsutil/zip.go` functions
- Integration tests: backup → zip → restore round-trip
- Security: zip-slip path traversal rejection

---

## Unit Tests — `internal/fsutil/zip_test.go`

### ZipDir

- [ ] **T1.1** `TestZipDir_RoundTrip` — zip a tree, unzip, verify file contents identical
- [ ] **T1.2** `TestZipDir_PreservesNestedPaths` — file at `a/b/c.txt` stored as `a/b/c.txt` in zip
- [ ] **T1.3** `TestZipDir_SkipsSymlinks` — symlinks present but not included in zip
- [ ] **T1.4** `TestZipDir_AtomicWrite` — if walk fails mid-way, no `.zip` artifact remains at dest path
- [ ] **T1.5** `TestZipDir_EmptyDir` — empty source dir produces valid (empty) zip
- [ ] **T1.6** `TestZipDir_OverwritesExisting` — calling twice replaces first zip cleanly

### UnzipToTemp

- [ ] **T1.7** `TestUnzipToTemp_ExtractsFiles` — extracted files match originals
- [ ] **T1.8** `TestUnzipToTemp_RecreatesDirs` — nested dirs are created
- [ ] **T1.9** `TestUnzipToTemp_ZipSlipRejected` — entry `../../evil.sh` returns error, temp dir cleaned up
- [ ] **T1.10** `TestUnzipToTemp_CleanupRemovesDir` — cleanup() deletes temp dir
- [ ] **T1.11** `TestUnzipToTemp_InvalidZip` — non-zip file returns error

---

## Integration Tests — `internal/fsutil/zip_test.go`

- [ ] **T2.1** `TestZipDir_UnzipRoundTrip_WithManifest` — backup dir (with manifest.yaml + category dirs) → zip → unzip → read manifest, verify all file paths resolvable

---

## CLI Integration Tests (manual / end-to-end)

- [ ] **T3.1** `macback backup --zip` — produces `<dest>.zip`, uncompressed dir still present
- [ ] **T3.2** `macback backup --zip --zip-only` — produces `.zip`, uncompressed dir removed
- [ ] **T3.3** `macback restore -s backup.zip --force` — all files restored, no temp dir left behind
- [ ] **T3.4** `macback list -s backup.zip` — shows category/file counts from manifest inside zip
- [ ] **T3.5** `macback diff -s backup.zip` — compares zip backup against current system
- [ ] **T3.6** `macback restore -s backup.zip` (with encrypted files) — prompts passphrase, decrypts correctly
- [ ] **T3.7** Rotation: after 3 backups with `zip: true`, old `.zip` files cleaned up alongside dirs

---

## Test Data

- Temp dirs created with `t.TempDir()` — cleaned up automatically
- Synthetic zip entries with `archive/zip` writer for adversarial cases (zip-slip)

---

## Test Reporting & Coverage

```bash
make test-coverage   # generates coverage.html
# Target: fsutil/zip.go at 100% branch coverage
```

Coverage gaps to document if any:
- `ZipDir` mid-walk error path (hard to induce; acceptable with manual injection)
