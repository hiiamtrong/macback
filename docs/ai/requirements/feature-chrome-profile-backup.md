---
phase: requirements
title: "Chrome Profile Backup - Requirements"
description: Back up Chrome (and other Chromium-based browsers) user profiles including bookmarks, extensions, preferences, and history — excluding cache directories to keep backup size small.
---

# Chrome Profile Backup - Requirements

## Problem Statement

- Developer uses multiple Chromium-based browsers (Brave, Chrome, Edge, Arc, Vivaldi, Opera) with accumulated bookmarks, extensions, and preferences built up over years.
- When switching Macs or recovering from disk failure, re-setting up all browser profiles is tedious: re-installing extensions, re-pinning tabs, re-entering preferences.
- The Chrome profile directory is huge because it includes HTTP caches, GPU caches, and compiled code — naively copying it would produce GBs of useless data.
- Who is affected: Any developer / power user who cares about preserving their browser environment.
- Current workaround: Sync via Google/Brave account (limited), manual copy (includes cache), or no backup at all.

## Goals & Objectives

### Primary Goals
- Back up user profile data for all detected Chromium-based browsers on the system
- Smart exclusion of cache directories (Cache, Code Cache, GPUCache, DawnCache, etc.) so backup stays small
- Support multiple profiles per browser (Default, Profile 1, Profile 2, …)
- Configurable: user can choose which browsers and which profile data files to include/exclude

### Secondary Goals
- Support both well-known browsers (auto-detected) and custom paths via config
- Report per-browser file counts in backup summary
- Sensitive files (Cookies, Login Data) backed up as-is (they are already macOS Keychain-protected on disk)

### Non-Goals
- Decrypting or reading browser-encrypted data (Cookies, Login Data use OS-level encryption)
- Restoring profiles back into the browser (restore copies files; browser must be closed)
- Syncing across machines in real-time
- Backing up browser binaries or system-level browser files

## User Stories & Use Cases

1. As a developer, I want to **back up my Brave browser bookmarks and extensions** so I can restore them on a new Mac.
2. As a developer, I want **cache directories automatically excluded** so my backup doesn't grow to multiple GBs.
3. As a developer with **multiple profiles** (work, personal), I want all profiles backed up without manual configuration.
4. As a developer, I want to **configure which browsers** are included (skip browsers I don't care about).
5. As a developer, I want **extension data preserved** so my installed extensions restore correctly.

### Key Workflows
- `macback backup` → `browser` category detects installed browsers, backs up all profiles minus cache
- `macback restore` → copies profile files back (user must quit browser first)
- `macback diff` → shows which profile files would be updated

### Edge Cases
- Browser not installed → skip with warning, no error
- Profile directory locked by running browser → copy will partially fail; warn per file
- Very large IndexedDB or Extension data → respect `max_file_size_mb` limit
- Multiple profiles: `Default`, `Profile 1`, `Profile 2` → all discovered automatically

## Success Criteria

- [ ] `browser` category added to config validation whitelist
- [ ] `BrowserHandler` implements `Category` interface (`Name`, `Discover`, `Backup`)
- [ ] Auto-detects at least: Brave, Chrome, Chromium, Edge, Arc, Vivaldi, Opera
- [ ] Excludes cache dirs: `Cache`, `Code Cache`, `GPUCache`, `DawnCache`, `DawnGraphiteCache`, `GraphiteDawnCache`, `GrShaderCache`, `CacheStorage`, `ScriptCache`, `PnaclTranslationCache`, `ShaderCache`
- [ ] Discovers all profiles (`Default`, `Profile *`, `Guest Profile`) under each browser
- [ ] `max_file_size_mb` respected (skip huge IndexedDB blobs)
- [ ] Backed up files preserve relative path: `browser/<BrowserName>/<ProfileName>/Bookmarks`
- [ ] Unit tests: discovery, exclusion, multi-profile, missing browser
- [ ] Default config entry: enabled, all major browsers listed, cache excluded

## Constraints & Assumptions

### Technical Constraints
- macOS only — browser data at `~/Library/Application Support/<BrowserVendorPath>/`
- Browser should be quit before restore to avoid file conflicts (documented, not enforced)
- Encryption of `Login Data` / `Cookies` is handled by macOS Keychain; macback does NOT re-encrypt these — they are safe to copy as opaque blobs
- Uses existing `Category` interface — no engine changes

### Assumptions
- User's home directory contains `~/Library/Application Support/`
- Profile directories follow Chromium naming convention (`Default`, `Profile N`, `Guest Profile`)
- Extensions directory structure: `Extensions/<ext-id>/<version>/`

## Questions & Open Items

- [x] Which browsers to auto-detect? → Brave, Chrome, Chromium, Microsoft Edge, Arc, Vivaldi, Opera
- [x] Should Login Data / Cookies be treated as secrets (encrypted with age)? → **No** — they are already OS-encrypted opaque blobs; re-encrypting with age is redundant and would break them. Back up as-is.
- [x] What profile naming pattern to discover? → Walk top-level dirs matching `Default` or `Profile *` or `Guest Profile`
- [ ] Should `History` be included? Could be privacy-sensitive. → Default: **yes**, user can exclude via config
- [ ] Should `Service Worker/` subtree be included? Contains offline app data. → Default: **yes, but skip CacheStorage subtree**
