package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/hiiamtrong/macback/internal/backup"
	"github.com/hiiamtrong/macback/internal/crypto"
	"github.com/hiiamtrong/macback/internal/logger"
	"github.com/hiiamtrong/macback/internal/restore"
)

func newDiffCmd() *cobra.Command {
	var source string
	var categories string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between backup and current system",
		Long:  "Compare a backup folder or .zip archive with the current system state.",
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

			var categoryFilter []string
			if categories != "" {
				categoryFilter = strings.Split(categories, ",")
			}

			log := logger.New(verbose)
			engine := restore.NewEngine(&crypto.NullDecryptor{}, log)
			diffs, err := engine.Diff(context.Background(), manifest, backupDir, categoryFilter, false)
			if err != nil {
				return err
			}

			restore.PrintDiffs(diffs)
			return nil
		},
	}

	cmd.Flags().StringVarP(&source, "source", "s", "", "backup source folder or .zip archive (required)")
	cmd.Flags().StringVar(&categories, "categories", "", "comma-separated categories")
	_ = cmd.MarkFlagRequired("source")

	return cmd
}
