package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/hiiamtrong/macback/internal/backup"
	"github.com/hiiamtrong/macback/internal/crypto"
	"github.com/hiiamtrong/macback/internal/logger"
	"github.com/hiiamtrong/macback/internal/restore"
)

func newRestoreCmd() *cobra.Command {
	var source string
	var categories string
	var force bool
	var dryRun bool
	var passphraseFile string

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore configuration files from backup",
		Long:  "Restore macOS configuration files from a backup folder or .zip archive.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return fmt.Errorf("--source is required")
			}

			backupDir, cleanup, err := resolveBackupSource(source)
			if err != nil {
				return err
			}
			defer cleanup()

			manifest, err := backup.ReadManifest(backupDir)
			if err != nil {
				return fmt.Errorf("reading backup manifest: %w", err)
			}

			fmt.Printf("Backup from: %s (%s)\n", manifest.CreatedAt.Format("2006-01-02 15:04:05"), manifest.Hostname)

			var categoryFilter []string
			if categories != "" {
				categoryFilter = strings.Split(categories, ",")
			}

			var dec crypto.Decryptor
			if manifest.HasEncryptedFiles() {
				dec, err = getDecryptorWithRetry(manifest, backupDir, passphraseFile)
				if err != nil {
					return err
				}
			} else {
				dec = &crypto.NullDecryptor{}
			}

			log := logger.New(verbose)
			engine := restore.NewEngine(dec, log)

			if dryRun {
				diffs, err := engine.Diff(context.Background(), manifest, backupDir, categoryFilter)
				if err != nil {
					return err
				}
				restore.PrintDiffs(diffs)
				return nil
			}

			result, err := engine.Run(context.Background(), manifest, backupDir, categoryFilter, force)
			if err != nil {
				return err
			}

			// Patch browser Local State files so all restored profile directories
			// are registered in Chrome's profile picker, even for backups made
			// before automatic patching was introduced.
			backup.PatchAllLocalStates()

			fmt.Printf("\nRestore complete.\n")
			fmt.Printf("  Restored: %d files\n", result.Restored)
			fmt.Printf("  Skipped:  %d files\n", result.Skipped)
			fmt.Printf("  Errors:   %d\n", result.Errors)
			return nil
		},
	}

	cmd.Flags().StringVarP(&source, "source", "s", "", "backup source folder or .zip archive (required)")
	cmd.Flags().StringVar(&categories, "categories", "", "comma-separated categories to restore")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompts")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview what would be restored")
	cmd.Flags().StringVar(&passphraseFile, "passphrase-file", "", "read passphrase from file")
	_ = cmd.MarkFlagRequired("source")

	return cmd
}

func getDecryptorWithRetry(manifest *backup.Manifest, backupDir string, passphraseFile string) (crypto.Decryptor, error) {
	// Find the first encrypted file for testing
	var testFile string
	for _, cat := range manifest.Categories {
		for _, f := range cat.Files {
			if f.Encrypted {
				testFile = filepath.Join(backupDir, f.Path)
				break
			}
		}
		if testFile != "" {
			break
		}
	}

	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		passphrase, err := getPassphrase(passphraseFile, "Enter passphrase for decrypting secrets: ")
		if err != nil {
			return nil, err
		}

		dec := crypto.NewPassphraseDecryptor(passphrase)

		// If no test file or reading from file, trust the input
		if testFile == "" || passphraseFile != "" {
			return dec, nil
		}

		// Try decrypting the test file to verify passphrase
		tmpDir, _ := os.MkdirTemp("", "macback-test-*")
		testDst := filepath.Join(tmpDir, "test")
		err = dec.DecryptFile(testFile, testDst)
		_ = os.RemoveAll(tmpDir)

		if err == nil {
			return dec, nil
		}

		if i < maxRetries-1 {
			fmt.Println("Wrong passphrase. Try again.")
		}
	}

	return nil, fmt.Errorf("wrong passphrase after %d attempts", maxRetries)
}
