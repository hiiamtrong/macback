package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/hiiamtrong/macback/internal/backup"
	"github.com/hiiamtrong/macback/internal/config"
	"github.com/hiiamtrong/macback/internal/crypto"
	"github.com/hiiamtrong/macback/internal/fsutil"
	"github.com/hiiamtrong/macback/internal/logger"
	"golang.org/x/term"
)

func newBackupCmd() *cobra.Command {
	var dest string
	var categories string
	var dryRun bool
	var passphraseFile string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up configuration files",
		Long:  "Back up macOS configuration files to the specified destination folder.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(getConfigPath())
			if err != nil {
				return err
			}

			if dest != "" {
				cfg.BackupDest = dest
			}

			expandedDest, err := fsutil.ExpandPath(cfg.BackupDest)
			if err != nil {
				return fmt.Errorf("expanding backup destination: %w", err)
			}

			var categoryFilter []string
			if categories != "" {
				categoryFilter = strings.Split(categories, ",")
			}

			var enc crypto.Encryptor
			enc = &crypto.NullEncryptor{}

			log := logger.New(verbose)
			engine := backup.NewEngine(cfg, enc, log)

			if dryRun {
				entries, err := engine.DryRun(context.Background(), categoryFilter)
				if err != nil {
					return err
				}
				fmt.Printf("Would back up %d files:\n", len(entries))
				for _, e := range entries {
					secret := ""
					if e.IsSecret {
						secret = " [ENCRYPTED]"
					}
					fmt.Printf("  %s%s\n", fsutil.ContractPath(e.SourcePath), secret)
				}
				return nil
			}

			// Set up encryption for actual backup
			if cfg.Encryption.Enabled {
				passphrase, err := getPassphraseConfirmed(passphraseFile)
				if err != nil {
					return err
				}
				enc = crypto.NewPassphraseEncryptor(passphrase)
				engine = backup.NewEngine(cfg, enc, log)
			}

			manifest, err := engine.Run(context.Background(), categoryFilter, expandedDest)
			if err != nil {
				return err
			}

			fmt.Printf("\nBackup complete to: %s\n", fsutil.ContractPath(expandedDest))
			fmt.Printf("Total files: %d\n", manifest.TotalFiles())
			fmt.Printf("Encrypted: %d\n", manifest.TotalEncrypted())
			return nil
		},
	}

	cmd.Flags().StringVarP(&dest, "dest", "d", "", "backup destination (overrides config)")
	cmd.Flags().StringVar(&categories, "categories", "", "comma-separated categories to back up")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be backed up without doing it")
	cmd.Flags().StringVar(&passphraseFile, "passphrase-file", "", "read passphrase from file")

	return cmd
}

func getPassphraseConfirmed(passphraseFile string) (string, error) {
	if passphraseFile != "" {
		// From file - no confirmation needed
		return getPassphrase(passphraseFile, "")
	}

	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		passphrase, err := getPassphrase("", "Enter passphrase for encrypting secrets: ")
		if err != nil {
			return "", err
		}
		confirm, err := getPassphrase("", "Confirm passphrase: ")
		if err != nil {
			return "", err
		}
		if passphrase == confirm {
			return passphrase, nil
		}
		fmt.Println("Passphrases do not match. Try again.")
	}
	return "", fmt.Errorf("passphrases did not match after %d attempts", maxRetries)
}

func getPassphrase(passphraseFile string, prompt string) (string, error) {
	if passphraseFile != "" {
		data, err := os.ReadFile(passphraseFile)
		if err != nil {
			return "", fmt.Errorf("reading passphrase file: %w", err)
		}
		lines := strings.SplitN(string(data), "\n", 2)
		return strings.TrimSpace(lines[0]), nil
	}

	fmt.Print(prompt)
	passBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("reading passphrase: %w", err)
	}
	fmt.Println()
	return string(passBytes), nil
}
