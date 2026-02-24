---
phase: testing
title: "Project Backup - Testing Strategy"
description: Test plan for the projects backup category handler
---

# Project Backup - Testing Strategy

## Test Coverage Goals

- 100% of new handler code (`internal/backup/projects.go`)
- 100% of new config fields in `internal/config/config.go` and `defaults.go`
- Integration test: full backup cycle for a synthetic project tree
- All error paths (missing dir, size exceeded, symlink, ctx cancel)

## Unit Tests (`internal/backup/projects_test.go`)

### ProjectsHandler — Discover

- [ ] **TestProjectsDiscover_BasicScan**
  - Setup: create `scanDir/myproject/{main.go, README.md, go.mod}`
  - Assert: 3 entries discovered, RelPaths = `myproject/main.go`, etc.

- [ ] **TestProjectsDiscover_ExcludesNodeModules**
  - Setup: add `scanDir/myproject/node_modules/lodash/index.js`
  - Assert: `node_modules` files NOT in entries; `node_modules` dir causes `SkipDir`

- [ ] **TestProjectsDiscover_ExcludesAllEcosystems**
  - Setup: create dirs: `vendor/`, `.venv/`, `__pycache__/`, `target/`, `dist/`, `.gradle/`, `build/`, `.next/`, `Pods/`
  - Assert: none of these dirs or their contents appear in entries

- [ ] **TestProjectsDiscover_SizeFilter**
  - Setup: create a 1-byte file and a 60 MB file (write sparse), set `MaxFileSizeMB: 50`
  - Assert: small file in entries; large file not in entries

- [ ] **TestProjectsDiscover_SizeFilterZeroMeansUnlimited**
  - Setup: `MaxFileSizeMB: 0`, create any large file
  - Assert: large file IS in entries

- [ ] **TestProjectsDiscover_MultipleScandirs**
  - Setup: `scanDir1/projectA/file.go`, `scanDir2/projectB/file.py`
  - Assert: both files discovered, RelPaths correct for each scanDir

- [ ] **TestProjectsDiscover_MissingScandirSkipped**
  - Setup: scanDir points to non-existent path
  - Assert: no panic, returns empty entries (warning handled by Backup)

- [ ] **TestProjectsDiscover_ProjectDepth2**
  - Setup: `scanDir/company/projectA/main.go`, `ProjectDepth: 2`
  - Assert: `company/projectA/main.go` in entries

- [ ] **TestProjectsDiscover_SymlinksSkipped**
  - Setup: create a symlink inside project
  - Assert: symlink not in entries

- [ ] **TestProjectsDiscover_DotEnvDetectedAsSecret**
  - Setup: project with `.env` file, `SecretPatterns: [".env"]`
  - Assert: `.env` entry has `IsSecret=true` after engine marks it (test via engine.Run integration or mock)

### ProjectsHandler — Backup

- [ ] **TestProjectsBackup_CopiesFiles**
  - Setup: entries with real source files, NullEncryptor, temp dest dir
  - Assert: all files exist in dest at expected relative paths

- [ ] **TestProjectsBackup_EncryptsSecrets**
  - Setup: entry with `IsSecret=true`, PassphraseEncryptor
  - Assert: `.age` file exists in dest, original not present unencrypted

- [ ] **TestProjectsBackup_WarnsOnCopyError**
  - Setup: entry pointing to non-existent source
  - Assert: result has warning, FileCount not incremented, no panic

- [ ] **TestProjectsBackup_ContextCancellation**
  - Setup: cancelled context
  - Assert: returns `ctx.Err()`

## Integration Tests (`internal/backup/projects_test.go`)

- [ ] **TestProjectsBackup_FullCycle**
  - Setup: synthetic project tree with 5 files + node_modules + .env
  - Run: `Discover()` then `Backup()` with NullEncryptor to temp dest
  - Assert:
    - `node_modules` absent from dest
    - `.env` present (NullEncryptor keeps it as-is)
    - All source files present at correct relative paths
    - `result.FileCount` matches expected non-excluded file count

- [ ] **TestProjectsConfig_DefaultsLoadable**
  - Load default config via `config.DefaultConfig()`
  - Assert: `projects` category exists, `enabled: false`, exclude list non-empty, `MaxFileSizeMB: 50`

## Config Unit Tests (`internal/config/config_test.go`)

- [ ] **TestValidate_AcceptsProjectsCategory** — config with `"projects"` doesn't fail validation
- [ ] **TestValidate_MaxFileSizeMBDefaultsToZero** — field absent → zero value (no limit)
- [ ] **TestValidate_ProjectDepthDefaultsToZero** — zero value handled gracefully by handler

## Test Data

- All test data created via `t.TempDir()` — no permanent fixtures needed
- Large file simulation: `os.Truncate` (sparse file) to avoid actual 60 MB writes
- Symlink creation: `os.Symlink()`

## Test Reporting & Coverage

```bash
# Run all tests with coverage
go test -v -race -cover ./internal/backup/... ./internal/config/...

# Generate HTML report
go test -coverprofile=cover.out ./... && go tool cover -html=cover.out
```

Coverage target: 100% of `projects.go`, ≥90% of modified config files.

## Manual Testing

- [ ] Run `macback init`, open `~/.macback.yaml`, verify `projects` section is present and disabled
- [ ] Enable `projects`, set `scan_dirs: [~/Works]`, run `macback backup --dest /tmp/test-backup --categories projects`
- [ ] Confirm `node_modules` absent from output dest
- [ ] Confirm `.env` files are encrypted (`.age` extension)
- [ ] Run `macback list --source /tmp/test-backup` — verify projects listed

## Bug Tracking

- Tag project-backup bugs with label `category:projects`
- Regression: re-run `TestProjectsBackup_FullCycle` on every change to `projects.go`
