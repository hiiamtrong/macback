---
phase: requirements
title: "Project Backup - Requirements"
description: Scan all developer project directories and back up source code, excluding package install directories (node_modules, vendor, .venv, etc.)
---

# Project Backup - Requirements

## Problem Statement
**What problem are we solving?**

- Developers have many projects scattered across directories (e.g., `~/Works`, `~/projects`). When switching machines or recovering from disk failure, project source code and config files must be manually re-cloned or copied.
- Naively copying project directories includes enormous, re-installable artifacts: `node_modules` (can be GBs), `vendor/`, `.venv/`, build outputs, caches — making backups slow and unnecessarily large.
- Who is affected: Any developer who wants to back up project source code alongside their macOS configs.
- Current workaround: Manual `rsync` with exclusion flags, or keeping everything in git and hoping all repos are pushed.

## Goals & Objectives

### Primary Goals
- Scan one or more configured root directories (e.g., `~/Works`, `~/projects`) and back up all project files
- Auto-exclude package install and build artifact directories across all ecosystems (Node, Python, Go, Java, Rust, etc.)
- Configurable: users can add extra root dirs and extra exclude patterns in `~/.macback.yaml`
- Size-aware: skip files above a configurable maximum size (default 50 MB) to avoid accidentally backing up binaries or large assets

### Secondary Goals
- Respect `.gitignore` files within each project for smarter exclusion
- Optionally catalog-only mode (just record what projects exist, don't copy files)
- Report project count and total size backed up in summary output

### Non-Goals
- Backing up compiled binaries or build output (explicitly excluded)
- Cloud/remote upload (out of scope for macback overall)
- Real-time sync or watch mode
- Full git history (only working tree files)

## User Stories & Use Cases

1. As a developer, I want to **configure my project root directories** in `~/.macback.yaml` so macback knows where my projects live.
2. As a developer, I want to **back up all project source files** without `node_modules` or `.venv` so the backup is small and fast.
3. As a developer, I want to **automatically skip build artifacts** (dist/, target/, __pycache__) so I only back up what I wrote.
4. As a developer, I want to **add custom exclusion patterns** for my specific stacks so I have control over what's included.
5. As a developer, I want to **set a max file size limit** so I never accidentally back up a 500 MB database dump.
6. As a developer, I want to **see a summary** (N projects, M files, X MB backed up) so I know the backup worked.

### Key Workflows
- Configure once: add `projects` category to `~/.macback.yaml` with `scan_dirs: [~/Works, ~/projects]`
- Run `macback backup` — projects category is included automatically
- Restore on new machine: `macback restore --source ~/my-backup` restores project files

### Edge Cases
- Project root dir doesn't exist → skip with warning
- Deeply nested `node_modules` inside a monorepo → still excluded by pattern match at any depth
- Project has binary files (images, fonts) below max size → backed up normally
- Symlinks inside a project → skipped (consistent with other categories)
- Hidden project dirs (`.dotproject/`) → included unless explicitly excluded

## Success Criteria

- [ ] `projects` category added to `CategoryConfig` validation whitelist
- [ ] `ProjectsHandler` implements the `Category` interface (`Name`, `Discover`, `Backup`)
- [ ] Default exclude list covers: `node_modules`, `vendor`, `.venv`, `venv`, `env`, `__pycache__`, `.gradle`, `build`, `target`, `dist`, `.next`, `.nuxt`, `.cache`, `Pods`, `.git`
- [ ] Files above `max_file_size_mb` (default 50) are skipped with a warning
- [ ] Multiple `scan_dirs` are walked recursively with depth limit (default: detect project root = first non-excluded subdir)
- [ ] `project_depth` config option controls how many directory levels below `scan_dirs` to treat as project roots (default: 1)
- [ ] Unit tests cover: discovery with excludes, size filtering, multi-dir scanning
- [ ] Integration test: full backup of a synthetic project tree, verify node_modules not present in output
- [ ] Default config includes `projects` category (disabled by default — user must enable and configure `scan_dirs`)

## Constraints & Assumptions

### Technical Constraints
- Must integrate with existing `Category` interface — no changes to engine
- Uses `fsutil.CopyFile` / `fsutil.CopyDir` for actual file copying (consistent with other categories)
- `scan_dirs` paths support `~` expansion via `fsutil.ExpandPath`
- Category name: `"projects"`

### Assumptions
- Users will configure `scan_dirs` themselves — no auto-discovery of project roots
- Projects are flat under scan_dirs at depth 1 (e.g., `~/Works/projectA`, `~/Works/projectB`) by default
- Users understand this backs up the working tree, not git history

## Questions & Open Items

- [x] Should `.gitignore` be respected? → **Nice-to-have in v2**, not in initial implementation (adds complexity)
- [x] Default enabled or disabled? → **Disabled by default** — user must configure `scan_dirs` first
- [x] Max file size: configurable per category or global? → **Per category** via `max_file_size_mb` field on `CategoryConfig`
- [ ] Should `projects` category support encryption of secrets within projects (`.env` files)? → TBD, likely yes via existing `secret_patterns` mechanism
