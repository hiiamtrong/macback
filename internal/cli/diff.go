package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trongdev/macos-backup/internal/backup"
	"github.com/trongdev/macos-backup/internal/crypto"
	"github.com/trongdev/macos-backup/internal/fsutil"
	"github.com/trongdev/macos-backup/internal/logger"
	"github.com/trongdev/macos-backup/internal/restore"
)

func newDiffCmd() *cobra.Command {
	var source string
	var categories string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between backup and current system",
		Long:  "Compare a backup folder with the current system state.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return fmt.Errorf("--source is required")
			}

			expandedSource, err := fsutil.ExpandPath(source)
			if err != nil {
				return fmt.Errorf("expanding source path: %w", err)
			}

			manifest, err := backup.ReadManifest(expandedSource)
			if err != nil {
				return fmt.Errorf("reading backup manifest: %w", err)
			}

			var categoryFilter []string
			if categories != "" {
				categoryFilter = strings.Split(categories, ",")
			}

			log := logger.New(verbose)
			engine := restore.NewEngine(&crypto.NullDecryptor{}, log)
			diffs, err := engine.Diff(context.Background(), manifest, expandedSource, categoryFilter)
			if err != nil {
				return err
			}

			restore.PrintDiffs(diffs)
			return nil
		},
	}

	cmd.Flags().StringVarP(&source, "source", "s", "", "backup source folder (required)")
	cmd.Flags().StringVar(&categories, "categories", "", "comma-separated categories")
	_ = cmd.MarkFlagRequired("source")

	return cmd
}
