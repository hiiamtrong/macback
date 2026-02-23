# macback - macOS Config Backup & Restore Tool

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://img.shields.io/badge/CI-passing-brightgreen.svg)]()

A single-binary CLI tool that backs up macOS configuration files (SSH keys, shell configs, git settings, dotfiles, Homebrew packages, app preferences) to a folder with [age](https://github.com/FiloSottile/age)-encrypted secrets, and restores them easily.

## Installation

### Using `go install`

```bash
go install github.com/trongdev/macos-backup/cmd/macback@latest
```

### Binary download

Download the latest release from the [Releases](https://github.com/trongdev/macos-backup/releases) page:

- `macback-darwin-arm64` (Apple Silicon)
- `macback-darwin-amd64` (Intel)

### Build from source

```bash
git clone https://github.com/trongdev/macos-backup.git
cd macos-backup
make build
make install   # copies binary to /usr/local/bin/
```

## Quick Start

```bash
macback init                              # Create ~/.macback.yaml
macback backup                            # Back up everything
macback backup --dry-run                  # Preview what would be backed up
macback diff -s ~/macback-backups         # Compare backup vs current system
macback restore -s ~/macback-backups      # Restore from backup
macback list -s ~/macback-backups         # List backup contents
```

## Commands

| Command | Description | Key Flags |
|---------|-------------|-----------|
| `init` | Generate default config file | `-o` (output path), `--force` (overwrite existing) |
| `backup` | Back up configuration files | `-d` (destination), `--categories`, `--dry-run`, `--passphrase-file` |
| `restore` | Restore from a backup folder | `-s` (source, required), `--categories`, `--force`, `--dry-run`, `--passphrase-file` |
| `diff` | Compare backup vs current system | `-s` (source, required), `--categories` |
| `list` | List contents of a backup | `-s` (source, required), `--categories`, `--show-secrets` |
| `version` | Show version information | |
| `completion` | Generate shell completion scripts | `bash`, `zsh`, `fish` |

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config file (default: `~/.macback.yaml`) |
| `--verbose` | `-v` | Enable verbose output |

## Categories

macback organizes backups into categories. Each category can be independently enabled/disabled and configured.

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
| `mas` | Mac App Store apps list | Disabled (requires [`mas`](https://github.com/mas-cli/mas) CLI) |

## Encryption

macback uses [age](https://github.com/FiloSottile/age) passphrase-based encryption (scrypt KDF) to protect sensitive files in your backups.

### How it works

- Files that match **secret patterns** are automatically encrypted during backup.
- Each category can define its own `secret_patterns` (e.g., SSH private keys, `.git-credentials`).
- Global patterns in `encryption.global_secret_patterns` apply across all categories (e.g., `*.pem`, `*.key`, `.env`).
- Encrypted files get the `.age` extension appended to their filename.
- During restore, you provide the same passphrase to decrypt these files.

### Secret pattern matching

Patterns are matched against file names using glob syntax:

- `id_*` matches SSH private keys like `id_rsa`, `id_ed25519`
- `!*.pub` excludes public keys from being treated as secrets
- `*.pem`, `*.key` match certificate and key files
- `.env`, `.env.*` match environment files

### Providing a passphrase

```bash
# Interactive prompt (default)
macback backup

# Read from file (for automation / CI)
macback backup --passphrase-file /path/to/passphrase.txt
macback restore -s ~/macback-backups --passphrase-file /path/to/passphrase.txt
```

## Configuration

macback reads its configuration from `~/.macback.yaml`. Generate the default config with:

```bash
macback init
```

### Example configuration

```yaml
# Where backups are stored
backup_dest: ~/macback-backups

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
      - ~/.bashrc
      - ~/.profile

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

  mas:
    enabled: false  # requires `mas` CLI installed

# Encryption settings
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
| `categories.<name>.enabled` | Enable or disable a category |
| `categories.<name>.paths` | Files and directories to back up (supports globs and `~`) |
| `categories.<name>.secret_patterns` | Patterns for files that should be encrypted (prefix with `!` to exclude) |
| `categories.<name>.exclude` | Patterns for files to skip |
| `encryption.enabled` | Master switch for encryption |
| `encryption.global_secret_patterns` | Secret patterns applied to all categories |
| `encryption.extension` | File extension for encrypted files (default: `.age`) |

## Examples

### Back up only specific categories

```bash
macback backup --categories ssh,git,shell
```

### Preview what would be backed up

```bash
macback backup --dry-run
```

### Back up to a custom location

```bash
macback backup -d /Volumes/USB/my-backup
```

### Encrypted backup with passphrase from file

```bash
echo "my-secret-passphrase" > ~/.macback-passphrase
chmod 600 ~/.macback-passphrase
macback backup --passphrase-file ~/.macback-passphrase
```

### Restore specific categories

```bash
macback restore -s ~/macback-backups --categories ssh,git
```

### Check differences before restoring

```bash
macback diff -s ~/macback-backups
```

### Force restore (skip confirmation prompts)

```bash
macback restore -s ~/macback-backups --force
```

### Generate shell completions

```bash
# Bash
source <(macback completion bash)

# Zsh
source <(macback completion zsh)

# Fish
macback completion fish | source
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
