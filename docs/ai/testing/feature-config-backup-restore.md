---
phase: testing
title: "Config Backup & Restore - Testing Strategy"
description: Testing approach for the macOS config backup CLI tool
---

# Config Backup & Restore - Testing Strategy

## Test Coverage Goals

- Unit test coverage target: 100% of new code
- Integration test scope: full backup-restore roundtrip
- End-to-end test: `macback init -> backup -> diff -> restore -> verify`

## Unit Tests

### Config Package
- [ ] Load valid YAML config
- [ ] Handle missing config file (return ErrConfigNotFound)
- [ ] Validate required fields (backup_dest)
- [ ] Expand `~` in paths to home directory
- [ ] Handle invalid YAML syntax
- [ ] Default values applied for missing optional fields

### FSUtil Package
- [ ] CopyFile preserves content
- [ ] CopyFile preserves permissions (0600, 0644, 0755)
- [ ] CopyDir copies recursively
- [ ] SHA256File returns correct hash
- [ ] ExpandPath handles `~`, `~/path`, absolute paths

### Crypto Package
- [ ] IsSecret matches `id_*` pattern
- [ ] IsSecret excludes `!*.pub` negation
- [ ] IsSecret matches global patterns (*.pem, .env)
- [ ] IsSecret returns false for non-matching files
- [ ] EncryptFile -> DecryptFile roundtrip matches original
- [ ] DecryptFile with wrong passphrase returns error
- [ ] Encrypt empty file works correctly
- [ ] Encrypt preserves content integrity (SHA256 match after decrypt)

### Backup Handlers
- [ ] Shell handler discovers existing shell config files
- [ ] Shell handler skips non-existent files with warning
- [ ] SSH handler identifies private keys as secrets
- [ ] SSH handler identifies public keys as non-secrets
- [ ] Git handler backs up .gitconfig and .gitignore_global
- [ ] Dotfiles handler respects exclude patterns
- [ ] Dotfiles handler handles recursive directories
- [ ] Homebrew handler executes correct brew commands (mock)
- [ ] PathBin handler scans directories and catalogs binaries
- [ ] PathBin handler detects binary origin (homebrew, go, manual)

### Manifest
- [ ] Marshal/unmarshal roundtrip preserves all fields
- [ ] Manifest version compatibility check
- [ ] File count and encrypted count are accurate

### Restore
- [ ] Diff detects identical files
- [ ] Diff detects modified files
- [ ] Diff detects new files (in backup but not on system)
- [ ] Restore copies files to correct original paths
- [ ] Restore preserves file permissions
- [ ] Restore decrypts .age files correctly
- [ ] Force mode skips all prompts

## Integration Tests

- [ ] Full backup-restore roundtrip: create temp files -> backup -> wipe -> restore -> verify
- [ ] Encrypted backup-restore: backup with encryption -> restore with passphrase -> verify content
- [ ] Selective backup: `--categories ssh,shell` only backs up those categories
- [ ] Selective restore: `--categories shell` only restores shell configs
- [ ] Dry-run backup: no files written to destination
- [ ] Dry-run restore: no files modified on system
- [ ] Manifest integrity: tamper with backed-up file, verify restore detects mismatch

## End-to-End Tests

- [ ] `macback init` creates config file at expected location
- [ ] `macback backup --dest <tmp>` creates correct directory structure
- [ ] `macback list --source <tmp>` shows all backed-up files
- [ ] `macback diff --source <tmp>` shows no differences after fresh backup
- [ ] `macback restore --source <tmp> --force` restores without prompts

## Test Data

### Fixtures
- Sample SSH config and keys (test-only, not real)
- Sample .zshrc, .bashrc with known content
- Sample .gitconfig
- Mock brew command output

### Test Helpers
- `TempHome(t)` - creates temp directory with sample config files
- `MockBrewCmd(output)` - returns exec.Cmd that outputs specified text
- Golden files in `testdata/golden/` for manifest and diff output

## Test Reporting & Coverage

### Commands
```bash
go test -v -race -cover ./...              # Run all tests with coverage
go test -v -race -coverprofile=cover.out ./...  # Generate coverage profile
go tool cover -html=cover.out              # View coverage in browser
```

### Coverage Gaps
- Homebrew handler: brew command execution tested with mocks only (no live brew)
- Interactive prompts: tested with stdin mocking

## Performance Testing

- Backup of typical config (~100 files) should complete in < 10 seconds
- Encryption overhead per file: < 100ms
- Manifest parsing for 1000-file backup: < 1 second
