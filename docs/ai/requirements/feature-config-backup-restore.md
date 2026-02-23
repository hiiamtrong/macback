---
phase: requirements
title: "Config Backup & Restore - Requirements"
description: Back up macOS config files (SSH, shell, git, dotfiles, Homebrew, PATH binaries) to a folder with encrypted secrets, and restore them easily
---

# Config Backup & Restore - Requirements

## Problem Statement
**What problem are we solving?**

- Developers spend significant time reconfiguring a new Mac or recovering from system issues: SSH keys, shell aliases, git settings, Homebrew packages, dotfiles, and custom scripts must all be set up manually.
- There's no simple, unified tool to back up all these configs to a single folder and restore them with one command.
- Sensitive files like SSH private keys need secure handling during backup/restore.
- Who is affected: Any developer who uses macOS and wants portable, restorable configs.
- Current workaround: Manual copying, scattered dotfile repos, or complex tools like chezmoi that may be overkill.

## Goals & Objectives
**What do we want to achieve?**

### Primary Goals
- One-command backup of all macOS config files to a user-specified folder
- One-command restore from that backup folder
- Encrypted storage of sensitive files (SSH private keys, .env, credentials)
- Homebrew package list export and restore
- Custom PATH binary cataloging

### Secondary Goals
- Selective backup/restore by category
- Diff/preview before restoring
- Dry-run mode for safety
- YAML-based configuration for customization

### Non-Goals
- Cross-platform support (macOS only)
- Cloud sync or remote backup storage
- Full system backup (only config files)
- GUI interface

## User Stories & Use Cases
**How will users interact with the solution?**

1. As a developer, I want to **initialize a config file** (`macback init`) so that I can customize what gets backed up.
2. As a developer, I want to **back up all my config files** to a folder so I can restore them on a new machine.
3. As a developer, I want **SSH private keys and .env files encrypted** with a passphrase so backups are safe to store.
4. As a developer, I want to **export my Homebrew packages** (formulae, casks, taps) and reinstall them from backup.
5. As a developer, I want to **catalog custom binaries** in my PATH so I know what to reinstall.
6. As a developer, I want to **preview changes before restoring** (`macback diff`) to avoid overwriting newer files.
7. As a developer, I want to **selectively backup/restore** specific categories (e.g., just SSH, just shell configs).
8. As a developer, I want to **list backup contents** (`macback list`) to see what's in a backup folder.

### Key Workflows
- **Fresh backup**: `macback init` -> edit config -> `macback backup --dest ~/my-backups`
- **Restore on new machine**: Install Go binary -> `macback restore --source ~/my-backups`
- **Check before restore**: `macback diff --source ~/my-backups`

### Edge Cases
- Source files don't exist (e.g., no ~/.bashrc) - skip with warning
- Destination folder doesn't exist - create it
- Encrypted file decryption with wrong passphrase - retry up to 3 times
- File on system is newer than backup - prompt user before overwriting

## Success Criteria
**How will we know when we're done?**

- [ ] `macback init` generates a default `~/.macback.yaml` config
- [ ] `macback backup` backs up all 6 categories (SSH, shell, git, dotfiles, homebrew, pathbin)
- [ ] Sensitive files are encrypted with age passphrase-based encryption
- [ ] `macback restore` restores files with interactive conflict resolution
- [ ] `macback diff` shows what would change before restoring
- [ ] `macback list` displays backup contents by category
- [ ] Manifest file tracks all backed-up files with metadata (hash, permissions, encryption status)
- [ ] Full backup-restore roundtrip passes in automated tests
- [ ] File permissions are preserved during backup and restore

## Constraints & Assumptions
**What limitations do we need to work within?**

### Technical Constraints
- macOS only (Darwin)
- Go 1.21+ required for build
- `filippo.io/age` Go library for encryption (no external `age` binary needed)
- Homebrew features require `brew` CLI to be installed
- `mas` CLI needed for Mac App Store backup (optional)

### Assumptions
- Users have Go installed or can download the pre-built binary
- Backup destination folder is accessible and writable
- Passphrase is memorized by the user (not stored anywhere)

## Questions & Open Items
**Resolved during requirements gathering:**

- [x] Language choice: **Go** (single binary, no runtime deps)
- [x] Encryption method: **age** passphrase-based (scrypt)
- [x] Backup destination: **Custom folder** (user-specified)
- [x] Secrets handling: **Encrypt with age**
- [x] Binary scanning: **Both** Homebrew list and PATH binary catalog
