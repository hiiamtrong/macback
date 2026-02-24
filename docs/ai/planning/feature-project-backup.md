---
phase: planning
title: "Project Backup - Task Breakdown"
description: Implementation plan for the projects backup category
---

# Project Backup - Task Breakdown

## Milestones

- [ ] **M1 — Config & Scaffolding**: `projects` category recognized in config, default config entry added
- [ ] **M2 — Core Handler**: `ProjectsHandler` discovers and backs up files correctly with exclusions and size filter
- [ ] **M3 — Tests Green**: Unit + integration tests pass, lint clean
- [ ] **M4 — Release**: Docs updated, changelog entry, version tag

## Task Breakdown

### Phase 1: Config Changes (`internal/config/`)

- [ ] **T1.1** Add `MaxFileSizeMB int` and `ProjectDepth int` fields to `CategoryConfig` in `config.go`
- [ ] **T1.2** Add `"projects"` to `validCategories` map in `config.go` `Validate()`
- [ ] **T1.3** Add default `projects` entry to `DefaultConfig()` in `defaults.go`:
  - `enabled: false`
  - `scan_dirs: [~/Works, ~/projects]`
  - `project_depth: 1`
  - `max_file_size_mb: 50`
  - Full default exclude list (node_modules, vendor, .venv, etc.)
  - `secret_patterns: [.env, .env.*, "*secret*", "*.pem", "*.key"]`
- [ ] **T1.4** Update config unit tests (`config_test.go`) to cover new fields and `projects` validation

### Phase 2: Handler Implementation (`internal/backup/projects.go`)

- [ ] **T2.1** Create `internal/backup/projects.go` with `ProjectsHandler` struct and `init()` registration
- [ ] **T2.2** Implement `Name() string` → returns `"projects"`
- [ ] **T2.3** Implement `Discover(cfg)`:
  - Expand each `scan_dir` with `fsutil.ExpandPath`
  - Warn and skip if dir doesn't exist
  - Walk to `project_depth` to enumerate project roots
  - For each project root: call `discoverProject()`
- [ ] **T2.4** Implement `discoverProject(scanDir, projectDir, cfg)`:
  - `filepath.WalkDir` the project dir
  - Call `shouldExcludeEntry()` (reuse from dotfiles.go or extract to `exclude.go`)
  - Skip files > `maxBytes(cfg)`; emit warning in result
  - Skip symlinks
  - Build `RelPath` as `filepath.Rel(scanDir, filePath)`
  - Return `[]FileEntry`
- [ ] **T2.5** Implement `Backup(ctx, entries, dest, enc)`:
  - Delegate to `BackupFileEntry()` per entry (consistent with SSH/dotfiles pattern)
  - Collect warnings, counts
- [ ] **T2.6** Extract `shouldExcludeEntry` to `internal/backup/exclude.go` (or keep in dotfiles.go and call from projects.go — same package)

### Phase 3: Tests (`internal/backup/projects_test.go`)

- [ ] **T3.1** `TestProjectsDiscover_BasicScan` — creates synthetic project tree, verifies correct files discovered
- [ ] **T3.2** `TestProjectsDiscover_ExcludesNodeModules` — node_modules directory not in entries
- [ ] **T3.3** `TestProjectsDiscover_ExcludesAllEcosystems` — vendor, .venv, __pycache__, target, dist, .gradle, Pods all excluded
- [ ] **T3.4** `TestProjectsDiscover_SizeFilter` — files above max_file_size_mb are not in entries
- [ ] **T3.5** `TestProjectsDiscover_MultipleScandirs` — two scan_dirs, all projects discovered
- [ ] **T3.6** `TestProjectsDiscover_MissingScandirWarns` — non-existent scan_dir produces warning, not crash
- [ ] **T3.7** `TestProjectsDiscover_ProjectDepth2` — project_depth=2 handles nested project structure
- [ ] **T3.8** `TestProjectsBackup_Integration` — full backup of synthetic tree, verify dest structure, verify node_modules absent
- [ ] **T3.9** `TestProjectsBackup_SecretPatterns` — .env files get encrypted if IsSecret=true

### Phase 4: Polish

- [ ] **T4.1** Update `macback init` output / help text to mention `projects` category
- [ ] **T4.2** Add `projects` to `README.md` features list (if exists)
- [ ] **T4.3** Run `golangci-lint run ./...` — fix any lint issues
- [ ] **T4.4** Run `go test ./...` — all tests green

## Dependencies

- T2.3 depends on T1.1 (config fields must exist before handler reads them)
- T2.4 depends on T2.6 (shouldExcludeEntry availability)
- T3.x depends on T2.x (handler must exist before tests)
- T4.x depends on T3.x (tests must pass before release)

## Timeline & Estimates

| Phase | Effort |
|-------|--------|
| Phase 1 (Config) | ~30 min |
| Phase 2 (Handler) | ~2 hours |
| Phase 3 (Tests) | ~1.5 hours |
| Phase 4 (Polish) | ~30 min |
| **Total** | **~4.5 hours** |

## Risks & Mitigation

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Large repos slow down backup | Medium | `max_file_size_mb` + exclude patterns; incremental backup already in engine |
| `shouldExcludeEntry` doesn't handle nested paths correctly | Low | Existing logic tested in dotfiles; add project-specific tests |
| User has nested `node_modules` 5 dirs deep | Low | `shouldExcludeEntry` matches at any path component depth |
| `project_depth=2` needed for monorepos but not obvious | Low | Document clearly in config comments; default=1 covers 90% of cases |

## Resources Needed

- Go 1.23+ (already required)
- `golangci-lint` (already set up)
- No new dependencies
