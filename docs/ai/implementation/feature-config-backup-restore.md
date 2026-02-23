---
phase: implementation
title: "Config Backup & Restore - Implementation Guide"
description: Technical implementation details for the macOS config backup CLI tool
---

# Config Backup & Restore - Implementation Guide

## Development Setup

### Prerequisites
- Go 1.21+ installed (`brew install go`)
- `golangci-lint` for linting (`brew install golangci-lint`)
- `make` for build automation

### Environment Setup
```bash
cd /Users/trongdev/Works/Personal/macos-backup
go mod init github.com/trongdev/macos-backup
go mod tidy
```

### Build & Run
```bash
make build          # Build binary to bin/macback
make test           # Run all tests
make lint           # Run linter
make install        # Install to /usr/local/bin
```

## Code Structure

```
cmd/macback/main.go           # Entry point - wires cobra root command
internal/
  cli/                        # CLI layer (thin cobra wrappers)
    root.go                   # Root command, persistent flags (--config, --verbose)
    init.go                   # macback init
    backup.go                 # macback backup
    restore.go                # macback restore
    diff.go                   # macback diff
    list.go                   # macback list
    version.go                # macback version
  config/                     # Configuration
    config.go                 # Config struct, Load(), Validate()
    defaults.go               # DefaultConfig() with all category defaults
  backup/                     # Backup domain
    category.go               # Category interface, Registry, FileEntry types
    engine.go                 # Engine.Run(), Engine.DryRun()
    manifest.go               # Manifest struct, ReadManifest(), WriteManifest()
    ssh.go                    # SSH category handler
    shell.go                  # Shell category handler
    git.go                    # Git category handler
    dotfiles.go               # Dotfiles category handler
    homebrew.go               # Homebrew category handler
    pathbin.go                # PATH binary scanner handler
  restore/                    # Restore domain
    engine.go                 # Engine.Run(), restore orchestration
    diff.go                   # Diff(), DiffEntry types
    safety.go                 # ConfirmRestore(), interactive prompts
  crypto/                     # Encryption
    age.go                    # PassphraseEncryptor, PassphraseDecryptor
    detect.go                 # IsSecret() pattern matching
  fsutil/                     # File utilities
    copy.go                   # CopyFile(), CopyDir(), SHA256File()
    homedir.go                # ExpandPath() - expand ~ to home dir
```

### Naming Conventions
- Package names: lowercase, single word (`config`, `backup`, `crypto`)
- Interface names: descriptive (`Category`, `Encryptor`, `Decryptor`)
- Error variables: `Err` prefix (`ErrConfigNotFound`, `ErrDecryptFailed`)
- Test files: `*_test.go` in same package

## Implementation Notes

### Core Features

#### Config Loading
- YAML loaded with `gopkg.in/yaml.v3`
- Path expansion: `~` -> `os.UserHomeDir()`
- Glob expansion: `filepath.Glob()` for patterns like `~/.ssh/id_*`
- Validation: check required fields, valid category names

#### Backup Engine
- Iterates registered categories (or filtered by `--categories` flag)
- For each category: Discover -> filter secrets -> Backup
- Generates manifest after all categories complete
- Non-fatal errors per file (skip and warn), fatal errors abort

#### Category Handlers
- Each implements `Category` interface
- SSH, Shell, Git, Dotfiles: file-based (Discover scans paths, Backup copies files)
- Homebrew: command-based (runs `brew bundle dump`, `brew list`)
- PathBin: scanner-based (walks directories, catalogs binaries)

#### Encryption Flow
1. `crypto.IsSecret(filename, patterns)` checks if file matches secret patterns
2. Negation with `!` prefix (e.g., `!*.pub` excludes public keys)
3. `PassphraseEncryptor.EncryptFile()` uses `age.Encrypt()` with `ScryptRecipient`
4. Encrypted files get `.age` extension
5. Passphrase prompted once per session via `term.ReadPassword()`

#### Restore Safety
- Read manifest, compare with current system state
- Show diff for modified files
- Prompt: overwrite / skip / diff / force
- Restore file permissions from manifest via `os.Chmod()`

### Patterns & Best Practices
- **Context propagation**: All long-running operations accept `context.Context`
- **Error wrapping**: Use `fmt.Errorf("category %s: %w", name, err)` for context
- **Testability**: Interfaces for crypto, use temp dirs in tests
- **No global state**: All state flows through function parameters

## Integration Points

### Homebrew
- Exec: `brew bundle dump --file=<path> --force`
- Exec: `brew list --formula -1`, `brew list --cask -1`, `brew tap`
- Restore: `brew bundle install --file=<path>`
- Use `os/exec.Command` with context and timeout

### age Encryption
- Library: `filippo.io/age`
- Encrypt: `age.Encrypt(writer, age.NewScryptRecipient(passphrase))`
- Decrypt: `age.Decrypt(reader, age.NewScryptIdentity(passphrase))`

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Config not found | Fatal: suggest `macback init` |
| Source file missing | Warning: skip, continue |
| Permission denied | Warning: skip, continue |
| Dest not writable | Fatal: exit code 1 |
| Wrong passphrase | Retry up to 3 times |
| Partial failure | Exit code 2, summary of successes/failures |

## Security Notes

### Encryption
- age scrypt-based passphrase encryption for all detected secrets
- SHA256 hash of original (unencrypted) content stored in manifest for integrity
- Passphrase held in memory only, never written to disk

### File Permissions
- Original permissions stored in manifest
- Restored exactly on restore (especially `0600` for SSH keys)
- No symlinks followed during restore

### Secret Detection
- Per-category patterns + global patterns
- Conservative: when in doubt, encrypt
- Negation patterns to exclude non-secrets (e.g., `!*.pub`)
