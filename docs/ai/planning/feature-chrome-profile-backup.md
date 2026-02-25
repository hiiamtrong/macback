---
phase: planning
title: "Chrome Profile Backup - Task Breakdown"
description: Implementation plan for the browser backup category
---

# Chrome Profile Backup - Task Breakdown

## Milestones

- [ ] **M1 — Config**: `browser` category recognized in config, default entry added
- [ ] **M2 — Handler**: `BrowserHandler` discovers profiles and backs up files correctly with cache exclusion
- [ ] **M3 — Tests Green**: Unit + integration tests pass, lint clean
- [ ] **M4 — Release**: Committed, pushed, tagged v0.3.0

## Task Breakdown

### Phase 1: Config Changes (`internal/config/`)

- [ ] **T1.1** Add `"browser"` to `validCategories` map in `config.go` `Validate()`
- [ ] **T1.2** Add default `browser` entry to `DefaultConfig()` in `defaults.go`:
  - `enabled: true`
  - `max_file_size_mb: 50`
  - `exclude: []` (built-in cache list is always applied in handler)
- [ ] **T1.3** Update config unit tests to cover `browser` validation and default entry

### Phase 2: Handler (`internal/backup/browser.go`)

- [ ] **T2.1** Create `internal/backup/browser.go` with `BrowserHandler` struct and `init()` registration
- [ ] **T2.2** Define `browserDef` struct `{Name, AppSupportPath string}` and `knownBrowsers()` returning list of 8 browsers
- [ ] **T2.3** Implement `isBuiltinCacheDir(name string) bool` — matches against hard-coded cache dir names
- [ ] **T2.4** Implement `discoverProfiles(browserDataDir string) []string` — returns dirs named `Default`, `Profile *`, `Guest Profile`
- [ ] **T2.5** Implement `discoverBrowserFiles(browserName, profileDir string, cfg) []FileEntry`:
  - `filepath.WalkDir` the profile dir
  - Skip entries where `isBuiltinCacheDir(d.Name())` → `filepath.SkipDir`
  - Apply user `cfg.Exclude` via `shouldExcludeEntry()`
  - Apply `MaxFileSizeMB` size filter
  - Build `RelPath` as `browser/<browserName>/<profileName>/<relToProfile>`
- [ ] **T2.6** Implement `Discover(cfg)`:
  - Iterate `knownBrowsers()`; expand `~/Library/Application Support/<path>`
  - Also iterate `cfg.ScanDirs` for custom browser paths
  - Skip browsers whose data dir doesn't exist
  - Call `discoverProfiles()` then `discoverBrowserFiles()` for each profile
- [ ] **T2.7** Implement `Backup(ctx, entries, dest, enc)` — same pattern as `dotfiles.go` (CopyFile per entry, collect warnings)

### Phase 3: Tests (`internal/backup/browser_test.go`)

- [ ] **T3.1** `TestBrowserDiscover_DetectsBrowser` — synthetic browser data dir, verifies files discovered
- [ ] **T3.2** `TestBrowserDiscover_ExcludesCacheDirs` — Cache/, Code Cache/, GPUCache/ not in entries
- [ ] **T3.3** `TestBrowserDiscover_MultipleProfiles` — Default + Profile 1 both discovered
- [ ] **T3.4** `TestBrowserDiscover_MissingBrowserSkipped` — non-existent browser dir → 0 entries, no crash
- [ ] **T3.5** `TestBrowserDiscover_UserExclude` — user adds `History` to exclude list → not in entries
- [ ] **T3.6** `TestBrowserDiscover_SizeFilter` — file > max_file_size_mb skipped
- [ ] **T3.7** `TestBrowserBackup_CopiesFiles` — full backup cycle, files at correct RelPaths
- [ ] **T3.8** `TestBrowserBackup_ContextCancellation` — cancelled ctx returns error
- [ ] **T3.9** `TestIsBuiltinCacheDir` — unit test for the cache dir matcher
- [ ] **T3.10** `TestDiscoverProfiles` — returns only Default/Profile N/Guest Profile dirs

### Phase 4: Polish

- [ ] **T4.1** Run `golangci-lint run ./...` — fix any issues
- [ ] **T4.2** Run `go test ./...` — all tests green
- [ ] **T4.3** Commit + push + tag `v0.3.0`

## Dependencies

- T2.5 depends on T2.3 (`isBuiltinCacheDir` must exist)
- T2.6 depends on T2.2, T2.4, T2.5
- T2.7 depends on T2.6
- T3.x depends on T2.x
- T4.x depends on T3.x

## Timeline & Estimates

| Phase | Effort |
|-------|--------|
| Phase 1 (Config) | ~20 min |
| Phase 2 (Handler) | ~1.5 hours |
| Phase 3 (Tests) | ~1.5 hours |
| Phase 4 (Polish) | ~20 min |
| **Total** | **~3.5 hours** |

## Risks & Mitigation

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Browser profile dir locked by running browser | Medium | Warn per file copy error; document "quit browser before restore" |
| New browser adds unknown cache dir name | Low | User can add to `exclude` in config; open issue to add to built-in list |
| IndexedDB contains huge blobs | Low | `max_file_size_mb` filter |
| Arc uses non-standard profile structure | Medium | Test against actual Arc data dir; may need profile discovery tweak |

## Resources Needed

- Go 1.23+ (already required)
- No new dependencies
