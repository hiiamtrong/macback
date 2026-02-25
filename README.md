# macback — macOS Config Backup & Restore

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/hiiamtrong/macback)](https://github.com/hiiamtrong/macback/releases)

A single-binary CLI tool that backs up macOS configuration, dotfiles, browser profiles, and project source code to a local folder with [age](https://github.com/FiloSottile/age)-encrypted secrets, and restores them easily on a new machine.

## Installation

### Binary download (recommended)

```bash
# Apple Silicon (M1/M2/M3)
curl -L https://github.com/hiiamtrong/macback/releases/latest/download/macback_$(curl -s https://api.github.com/repos/hiiamtrong/macback/releases/latest | grep tag_name | cut -d'"' -f4)_darwin_arm64.tar.gz | tar xz
sudo mv macback /usr/local/bin/

# Intel Mac
curl -L https://github.com/hiiamtrong/macback/releases/latest/download/macback_$(curl -s https://api.github.com/repos/hiiamtrong/macback/releases/latest | grep tag_name | cut -d'"' -f4)_darwin_amd64.tar.gz | tar xz
sudo mv macback /usr/local/bin/
```

Or download directly from the [Releases](https://github.com/hiiamtrong/macback/releases) page.

### Using `go install`

```bash
go install github.com/hiiamtrong/macback/cmd/macback@latest
```

### Build from source

```bash
git clone https://github.com/hiiamtrong/macback.git
cd macback
make build
make install   # copies binary to /usr/local/bin/
```

## Quick Start

```bash
macback init                              # Create ~/.macback.yaml
macback backup                            # Back up everything
macback backup --dry-run                  # Preview what would be backed up
macback backup --zip                      # Back up and compress to a .zip archive
macback backup --zip-only                 # Back up, compress to .zip, remove uncompressed dir
macback diff -s ~/macback-backups         # Compare backup vs current system
macback diff -s ~/macback-backups.zip     # Diff from a .zip archive
macback restore -s ~/macback-backups      # Restore from backup
macback restore -s ~/macback-backups.zip  # Restore from a .zip archive
macback list -s ~/macback-backups         # List backup contents
macback list -s ~/macback-backups.zip     # List contents of a .zip archive
macback bootstrap -s ~/macback-backups    # Generate setup script for a fresh machine
```

## Commands

| Command | Description | Key Flags |
|---------|-------------|-----------|
| `init` | Generate default config file | `-o` (output path), `--force` (overwrite) |
| `backup` | Back up configuration files | `-d` (destination), `--categories`, `--dry-run`, `--passphrase-file`, `--zip`, `--zip-only` |
| `restore` | Restore from a backup folder | `-s` (source or .zip, required), `--categories`, `--force`, `--dry-run`, `--passphrase-file` |
| `diff` | Compare backup vs current system | `-s` (source or .zip, required), `--categories` |
| `list` | List contents of a backup | `-s` (source or .zip, required), `--categories`, `--show-secrets` |
| `bootstrap` | Generate a setup script for a fresh machine | `-s` (source or .zip, required), `-o` (output path), `--run` |
| `version` | Show version information | |
| `completion` | Generate shell completion scripts | `bash`, `zsh`, `fish` |

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config file (default: `~/.macback.yaml`) |
| `--verbose` | `-v` | Enable verbose output |

## Categories

macback organizes backups into categories. Each can be independently enabled/disabled and configured.

| Category | What it backs up | Default |
|----------|-----------------|---------|
| `ssh` | SSH keys, config, known_hosts (`~/.ssh/`) | Enabled |
| `shell` | `.zshrc`, `.bashrc`, `.bash_profile`, `.profile`, `.zprofile`, `.zshenv`, `.aliases`, `.functions` | Enabled |
| `git` | `.gitconfig`, `.gitignore_global`, `.git-credentials` | Enabled |
| `dotfiles` | `~/.config/`, `~/bin/`, `~/scripts/`, `.vimrc`, `.tmux.conf` | Enabled |
| `homebrew` | Brewfile, formula/cask/tap lists | Enabled |
| `pathbin` | Catalog of custom binaries in PATH directories | Enabled |
| `appsettings` | `~/Library/Preferences/` plist files | Enabled |
| `apps` | `/Applications` catalog with source detection (App Store, Homebrew, etc.) | Enabled |
| `browser` | Chromium browser profiles (Brave, Chrome, Edge, Arc, Vivaldi, Opera…) | Enabled |
| `projects` | Developer project source code | Disabled (configure `scan_dirs` first) |
| `mas` | Mac App Store apps list | Disabled (requires [`mas`](https://github.com/mas-cli/mas) CLI) |

## Browser Backup

The `browser` category backs up profiles from all detected Chromium-based browsers:

- **Brave**, **Google Chrome**, **Chromium**, **Microsoft Edge**, **Arc**, **Vivaldi**, **Opera**, **OperaGX**

Each profile (`Default`, `Profile 1`, `Profile 2`, …) is backed up separately. The following are **always excluded** (ephemeral, rebuilt by the browser):

> `Cache`, `Code Cache`, `GPUCache`, `IndexedDB`, `File System`, `Shared Dictionary`, `DawnCache`, `DawnWebGPUCache`, `GrShaderCache`, `ShaderCache`, `CacheStorage`, `ScriptCache`, and related GPU/shader caches.

Default excludes (configurable — remove from `exclude` list to keep):

> `Extensions` (re-downloaded on sign-in), `Favicons`, `History`, `Local Storage`, `Sessions`, `WebStorage`

What is **kept** by default (the important stuff):

> `Bookmarks`, `Preferences`, `Login Data` (passwords), `Local Extension Settings` (MetaMask vault, extension data), `Sync Data`, `Cookies`, and all other profile data.

```yaml
browser:
  enabled: true
  max_file_size_mb: 50
  exclude:
    - Extensions     # remove to keep extension source files
    - History        # remove to keep browsing history
```

## Project Backup

The `projects` category scans developer directories and backs up source code, skipping package manager directories and build artifacts.

```yaml
projects:
  enabled: true
  scan_dirs:
    - ~/Works
    - ~/projects
  project_depth: 2        # depth to treat as project root (1 = ~/Works/myproject, 2 = ~/Works/Personal/myproject)
  max_file_size_mb: 50    # skip files larger than this
  exclude:
    - node_modules
    - vendor
    - .venv
    - venv
    - .git
    - build
    - dist
    - target
    - .next
    - data             # scraped/downloaded data
    - "*.mp4"          # video files
    - "*.db"           # database files
```

## Encryption

macback uses [age](https://github.com/FiloSottile/age) passphrase-based encryption (scrypt KDF) to protect sensitive files.

- Files matching **secret patterns** are encrypted during backup and get the `.age` extension.
- Each category defines its own `secret_patterns` (e.g., SSH private keys, `.git-credentials`).
- `encryption.global_secret_patterns` applies across all categories (e.g., `*.pem`, `*.key`, `.env`).
- Provide the same passphrase during restore to decrypt.

```bash
# Interactive prompt (default)
macback backup

# Read passphrase from file (for automation)
echo "my-passphrase" > ~/.macback-passphrase
chmod 600 ~/.macback-passphrase
macback backup --passphrase-file ~/.macback-passphrase
macback restore -s ~/macback-backups --passphrase-file ~/.macback-passphrase
```

## Configuration

Generate the default config with:

```bash
macback init
```

### Full example

```yaml
backup_dest: ~/macback-backups
max_backups: 3   # keep last N backups (0 = unlimited)

categories:
  ssh:
    enabled: true
    paths:
      - ~/.ssh/config
      - ~/.ssh/known_hosts
      - ~/.ssh/id_*
      - ~/.ssh/*.pub
    secret_patterns:
      - "id_*"
      - "!*.pub"
    exclude:
      - "*.sock"
      - "agent.*"

  shell:
    enabled: true
    paths:
      - ~/.zshrc
      - ~/.zprofile
      - ~/.zshenv
      - ~/.bashrc
      - ~/.bash_profile
      - ~/.profile
      - ~/.aliases
      - ~/.functions

  git:
    enabled: true
    paths:
      - ~/.gitconfig
      - ~/.gitignore_global
      - ~/.git-credentials
    secret_patterns:
      - ".git-credentials"

  dotfiles:
    enabled: true
    paths:
      - ~/.config/
      - ~/bin/
      - ~/.vimrc
      - ~/.tmux.conf
    exclude:
      - "*.log"
      - "Cache/"
      - "node_modules/"
    secret_patterns:
      - ".env"
      - ".env.*"
      - "*secret*"

  homebrew:
    enabled: true
    include_casks: true
    include_taps: true

  pathbin:
    enabled: true
    scan_dirs:
      - /usr/local/bin
      - ~/bin
      - ~/go/bin
      - ~/.local/bin
    catalog_only: true

  apps:
    enabled: true
    scan_dirs:
      - /Applications
      - ~/Applications
    catalog_only: true

  appsettings:
    enabled: true
    paths:
      - ~/Library/Preferences/
    exclude:
      - "com.apple.*"
      - "*.lockfile"

  browser:
    enabled: true
    max_file_size_mb: 50
    exclude:
      - Extensions
      - Favicons
      - History
      - Local Storage
      - Sessions
      - WebStorage

  projects:
    enabled: false   # set to true and configure scan_dirs
    scan_dirs:
      - ~/Works
      - ~/projects
    project_depth: 1
    max_file_size_mb: 50
    exclude:
      - node_modules
      - vendor
      - .venv
      - venv
      - .git
      - build
      - dist
      - target
      - .next
      - data
      - "*.mp4"
      - "*.db"
    secret_patterns:
      - ".env"
      - ".env.*"
      - "*.pem"
      - "*.key"

  mas:
    enabled: false  # requires `mas` CLI installed

encryption:
  enabled: true
  global_secret_patterns:
    - "*.pem"
    - "*.key"
    - "*.p12"
    - "*.pfx"
    - ".env"
    - ".env.*"
  extension: ".age"
```

### Key configuration fields

| Field | Description |
|-------|-------------|
| `backup_dest` | Directory where backups are stored (supports `~`) |
| `max_backups` | Number of backup snapshots to keep (0 = unlimited) |
| `categories.<name>.enabled` | Enable or disable a category |
| `categories.<name>.paths` | Files and directories to back up (supports globs and `~`) |
| `categories.<name>.secret_patterns` | Patterns for files that should be encrypted (prefix `!` to negate) |
| `categories.<name>.exclude` | Patterns for files/directories to skip |
| `categories.<name>.scan_dirs` | Directories to scan (for `browser`, `projects`, `apps`, `pathbin`) |
| `categories.projects.project_depth` | How many levels deep to find project roots |
| `categories.projects.max_file_size_mb` | Skip files larger than this size |
| `encryption.enabled` | Master switch for encryption |
| `encryption.global_secret_patterns` | Secret patterns applied to all categories |
| `encryption.extension` | Extension for encrypted files (default: `.age`) |

## Examples

```bash
# Back up only specific categories
macback backup --categories ssh,git,shell,browser

# Preview without copying
macback backup --dry-run

# Back up to external drive
macback backup -d /Volumes/USB/my-backup

# Check what changed since last backup
macback diff -s ~/macback-backups

# Restore specific categories on a new machine
macback restore -s ~/macback-backups --categories ssh,git,shell

# Force restore (skip confirmation prompts)
macback restore -s ~/macback-backups --force

# Shell completions
source <(macback completion zsh)
```

## Development

```bash
make build          # Build binary to bin/macback
make test           # Run tests with race detection
make test-coverage  # Generate HTML coverage report
make lint           # Run linter
make clean          # Remove build artifacts
make release        # Build for both arm64 and amd64
```

## License

MIT
