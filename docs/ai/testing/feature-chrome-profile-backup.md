---
phase: testing
title: "Chrome Profile Backup - Testing Strategy"
description: Test plan for the browser backup category handler
---

# Chrome Profile Backup - Testing Strategy

## Test Coverage Goals

- 100% of `browser.go` handler code
- All cache exclusion paths tested
- Multi-browser, multi-profile scenarios covered
- Error paths: missing browser, file locked, context cancel

## Unit Tests (`internal/backup/browser_test.go`)

### BrowserHandler — isBuiltinCacheDir

- [ ] **TestIsBuiltinCacheDir_CacheDirs** — `Cache`, `Code Cache`, `GPUCache`, `DawnCache`, `CacheStorage` all return true
- [ ] **TestIsBuiltinCacheDir_NonCacheDirs** — `Bookmarks`, `Extensions`, `Preferences`, `History` return false

### BrowserHandler — discoverProfiles

- [ ] **TestDiscoverProfiles_Standard** — dir with `Default/`, `Profile 1/`, `Profile 2/` → returns all 3
- [ ] **TestDiscoverProfiles_GuestProfile** — `Guest Profile/` dir included
- [ ] **TestDiscoverProfiles_IgnoresNonProfileDirs** — `Crashpad/`, `GrShaderCache/` not included
- [ ] **TestDiscoverProfiles_EmptyDir** — empty browser dir → 0 profiles

### BrowserHandler — Discover

- [ ] **TestBrowserDiscover_DetectsBrowser**
  - Setup: synthetic browser dir mimicking `~/Library/Application Support/BraveSoftware/Brave-Browser/Default/` with Bookmarks, Preferences, Extensions/
  - Assert: entries discovered with correct `RelPath` = `browser/BraveBrowser/Default/Bookmarks`

- [ ] **TestBrowserDiscover_ExcludesCacheDirs**
  - Setup: profile with `Cache/`, `Code Cache/`, `GPUCache/` dirs containing files
  - Assert: none of those files in entries; source files (Bookmarks) ARE in entries

- [ ] **TestBrowserDiscover_MultipleProfiles**
  - Setup: `Default/` + `Profile 1/` each with Bookmarks
  - Assert: both profiles' Bookmarks in entries with correct RelPaths

- [ ] **TestBrowserDiscover_MissingBrowserSkipped**
  - Handler uses real `HOME` path; none of the known browsers installed in test tmp
  - Override via `cfg.ScanDirs` pointing to temp dir; non-existent ScanDir → 0 entries, no crash

- [ ] **TestBrowserDiscover_UserExclude**
  - Setup: profile with `History`, `Bookmarks`; `cfg.Exclude = ["History"]`
  - Assert: History not in entries; Bookmarks IS

- [ ] **TestBrowserDiscover_SizeFilter**
  - Setup: `Bookmarks` (small) + `large.db` (sparse 60 MB); `MaxFileSizeMB: 50`
  - Assert: small file in entries; large file not

- [ ] **TestBrowserDiscover_CustomScanDir**
  - Setup: `cfg.ScanDirs` points to synthetic browser dir at custom path
  - Assert: files discovered with browser name derived from dir basename

### BrowserHandler — Backup

- [ ] **TestBrowserBackup_CopiesFiles**
  - Setup: entries with real source files, NullEncryptor, temp dest
  - Assert: files exist in dest at `<destDir>/browser/BraveBrowser/Default/Bookmarks`

- [ ] **TestBrowserBackup_WarnsOnCopyError**
  - Setup: entry pointing to non-existent source
  - Assert: warning emitted, FileCount=0, no panic

- [ ] **TestBrowserBackup_ContextCancellation**
  - Setup: cancelled context
  - Assert: returns `ctx.Err()`

## Integration Test

- [ ] **TestBrowserBackup_FullCycle**
  - Setup: synthetic browser dir with Default profile: Bookmarks, Preferences, Extensions/, Cache/ (with files)
  - Run: `Discover()` + `Backup()` with NullEncryptor to temp dest
  - Assert:
    - `Cache/` absent from dest
    - `Bookmarks` and `Preferences` present at correct paths
    - `result.FileCount` matches expected count

## Config Tests (`internal/config/config_test.go`)

- [ ] **TestValidate_AcceptsBrowserCategory** — config with `browser` doesn't fail validation
- [ ] **TestDefaultBrowserCategory** — default config has `browser` entry, `enabled: true`, `MaxFileSizeMB: 50`

## Test Data

- All synthetic browser dirs created via `t.TempDir()` — no real browser data touched
- Large file simulation: `os.Truncate` (sparse)
- Extensions dir: create `Extensions/<ext-id>/<version>/manifest.json`

## Test Reporting & Coverage

```bash
go test -v -race -cover ./internal/backup/... ./internal/config/...
go test -coverprofile=cover.out ./... && go tool cover -html=cover.out
```

Coverage target: 100% of `browser.go`, ≥90% of modified config files.

## Manual Testing

- [ ] Enable `browser` category in `~/.macback.yaml`
- [ ] Run `macback backup --dest /tmp/browser-test --categories browser --dry-run`
- [ ] Verify Bookmarks, Extensions visible in output; no Cache/ dirs
- [ ] Check file count is reasonable (< 10k files for a typical Brave profile)
- [ ] Run actual backup, verify dest structure matches `browser/<BrowserName>/<Profile>/...`
