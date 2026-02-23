---
phase: planning
title: "Config Backup & Restore - Planning"
description: Task breakdown and implementation order for the macOS config backup tool
---

# Config Backup & Restore - Planning

## Milestones

- [ ] Milestone 1: Foundation - Go module, config, fsutil, CLI skeleton
- [ ] Milestone 2: Core Backup - Backup engine + shell category proof of concept
- [ ] Milestone 3: Encryption - age crypto + SSH handler with encrypted secrets
- [ ] Milestone 4: Restore - Restore engine, diff, safety checks
- [ ] Milestone 5: Full Categories - Git, dotfiles, homebrew, pathbin handlers
- [ ] Milestone 6: Polish - Colors, dry-run, list command, comprehensive tests

## Task Breakdown

### Phase 1: Foundation
- [ ] Task 1.1: Initialize Go module (`go mod init`), create directory structure
- [ ] Task 1.2: Create Makefile with `build`, `test`, `lint`, `clean`, `install` targets
- [ ] Task 1.3: Implement `internal/config/config.go` - Config types, YAML loading, path expansion, validation
- [ ] Task 1.4: Implement `internal/config/defaults.go` - Default config with all categories
- [ ] Task 1.5: Write tests for config package
- [ ] Task 1.6: Implement `internal/fsutil/copy.go` - File copy with permissions, SHA256 hashing
- [ ] Task 1.7: Implement `internal/fsutil/homedir.go` - Home directory expansion
- [ ] Task 1.8: Write tests for fsutil package
- [ ] Task 1.9: Implement `internal/cli/root.go` - Root cobra command with persistent flags
- [ ] Task 1.10: Implement `internal/cli/init.go` - `macback init` generates default config
- [ ] Task 1.11: Implement `internal/cli/version.go` - Version command
- [ ] Task 1.12: Implement `cmd/macback/main.go` - Entry point

### Phase 2: Core Backup
- [ ] Task 2.1: Implement `internal/backup/category.go` - Category interface and registry
- [ ] Task 2.2: Implement `internal/backup/manifest.go` - Manifest struct and YAML serialization
- [ ] Task 2.3: Implement `internal/backup/engine.go` - Backup orchestration
- [ ] Task 2.4: Implement `internal/backup/shell.go` - Shell config handler
- [ ] Task 2.5: Write tests for backup package (shell category)
- [ ] Task 2.6: Implement `internal/cli/backup.go` - `macback backup` command
- [ ] Task 2.7: End-to-end test: `macback init && macback backup --categories shell`

### Phase 3: Encryption
- [ ] Task 3.1: Implement `internal/crypto/detect.go` - Secret pattern matching
- [ ] Task 3.2: Write tests for secret detection
- [ ] Task 3.3: Implement `internal/crypto/age.go` - Passphrase encrypt/decrypt with filippo.io/age
- [ ] Task 3.4: Write tests for age encryption (roundtrip, wrong passphrase)
- [ ] Task 3.5: Implement `internal/backup/ssh.go` - SSH handler with encryption
- [ ] Task 3.6: Write tests for SSH handler
- [ ] Task 3.7: Integration test: backup SSH with encryption, verify .age files

### Phase 4: Restore
- [ ] Task 4.1: Implement `internal/restore/engine.go` - Restore orchestration
- [ ] Task 4.2: Implement `internal/restore/diff.go` - File diff/comparison
- [ ] Task 4.3: Write tests for diff logic
- [ ] Task 4.4: Implement `internal/restore/safety.go` - Interactive prompts, conflict resolution
- [ ] Task 4.5: Implement `internal/cli/restore.go` - `macback restore` command
- [ ] Task 4.6: Implement `internal/cli/diff.go` - `macback diff` command
- [ ] Task 4.7: Integration test: full backup-then-restore roundtrip

### Phase 5: Remaining Categories
- [ ] Task 5.1: Implement `internal/backup/git.go` - Git config handler + tests
- [ ] Task 5.2: Implement `internal/backup/dotfiles.go` - Dotfiles handler (recursive, excludes) + tests
- [ ] Task 5.3: Implement `internal/backup/homebrew.go` - Homebrew handler (exec brew) + tests
- [ ] Task 5.4: Implement `internal/backup/pathbin.go` - PATH binary scanner + tests
- [ ] Task 5.5: Implement `internal/cli/list.go` - `macback list` command

### Phase 6: Polish
- [ ] Task 6.1: Add colored output with fatih/color
- [ ] Task 6.2: Implement `--dry-run` mode for backup and restore
- [ ] Task 6.3: Add cobra shell completion generation
- [ ] Task 6.4: Comprehensive integration tests for all categories
- [ ] Task 6.5: Final build verification and manual testing

## Dependencies

### Task Dependencies
- Phase 2 depends on Phase 1 (config, fsutil, CLI skeleton)
- Phase 3 depends on Phase 2 (backup engine needed for SSH handler)
- Phase 4 depends on Phase 2 + 3 (restore needs manifest and crypto)
- Phase 5 depends on Phase 2 (each handler uses backup engine)
- Phase 6 depends on all previous phases

### External Dependencies
- Go 1.21+ toolchain
- `filippo.io/age` Go module
- `github.com/spf13/cobra` Go module
- `gopkg.in/yaml.v3` Go module
- `golang.org/x/term` Go module
- `github.com/sergi/go-diff` Go module
- `github.com/fatih/color` Go module
- Homebrew CLI (for homebrew category, optional)

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| age library API changes | Build failure | Pin version in go.mod |
| Large ~/.config directories | Slow backup | Exclude patterns, progress output |
| Permission errors on sensitive files | Partial backup | Non-fatal warnings, continue with rest |
| Wrong passphrase on restore | Can't decrypt | Retry prompt (3 attempts), clear error message |
| Homebrew not installed | Category fails | Skip with warning, other categories proceed |

## Resources Needed

### Tools
- Go 1.21+ compiler
- `golangci-lint` for linting
- `make` for build automation

### Infrastructure
- Local filesystem for backup storage
- macOS system for testing
