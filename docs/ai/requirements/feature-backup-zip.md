---
phase: requirements
title: Backup ZIP Compression
description: Auto-compress backup output to a single .zip archive and restore from zip
---

# Requirements & Problem Understanding

## Problem Statement

After a successful backup, macback produces a directory containing hundreds or thousands of files
(`ssh/`, `browser/`, `projects/`, `manifest.yaml`, etc.). Transferring this to a new machine or
uploading to cloud storage requires handling many small files, which is error-prone and slow.

**Who is affected**: Any user who wants to transfer a backup to a new machine or store it remotely.

**Current workaround**: Manually run `zip -r backup.zip ~/macback-backups` after every backup.

## Goals & Objectives

**Primary goals**
- Produce a single `.zip` archive after backup, suitable for direct file transfer
- Allow `macback restore` to accept a `.zip` file as the source (auto-extracts)

**Secondary goals**
- Config option to enable zip by default
- Optionally keep the uncompressed directory alongside the zip (for incremental reuse)

**Non-goals**
- Other archive formats (`.tar.gz`, `.tar.bz2`) — zip is sufficient for macOS users
- Encryption of the zip container itself (age already handles per-file encryption)
- Cloud upload / push to remote storage

## User Stories & Use Cases

- As a user setting up a new Mac, I want to copy a single `.zip` file from my old machine and run
  `macback restore -s backup.zip` so I don't have to deal with a directory of thousands of files.

- As a user running scheduled backups, I want `zip: true` in my config so every backup
  automatically produces a `.zip` I can upload to iCloud Drive.

- As a user, I want `macback backup --zip` to produce both the directory (for the next incremental
  run) and a `.zip` I can archive.

## Success Criteria

- `macback backup --zip` completes without error and produces `<backup_dest>.zip`
- `macback restore -s /path/to/backup.zip --force` successfully restores all files
- `macback list -s /path/to/backup.zip` shows backup contents from the zip
- `macback diff -s /path/to/backup.zip` compares zip backup against current system
- Zip contains the same files as the uncompressed directory (verified by manifest)
- Existing backup/restore behaviour is unchanged when `--zip` is not used

## Constraints & Assumptions

- Use Go's standard `archive/zip` — no external tools required
- The zip structure mirrors the backup directory flat (no extra wrapping folder)
- Restore auto-detects `.zip` extension — no extra flag needed on the restore side
- For incremental backup: the uncompressed directory must remain for the next run; zip is
  created as an additional output (not a replacement)
- Max backup size is bounded by `max_file_size_mb` per file; total zip size is not capped

## Questions & Open Items

- Should the zip be created alongside the backup dir (keep both) or replace it?
  → **Decision**: keep both by default; add `--zip-only` flag to remove the dir after zipping.
- Should `max_backups` rotation also clean up old `.zip` files?
  → Yes — rotation logic should delete `<rotated_dir>.zip` when cleaning old backups.
