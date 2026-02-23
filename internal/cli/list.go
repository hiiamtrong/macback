package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trongdev/macos-backup/internal/backup"
	"github.com/trongdev/macos-backup/internal/fsutil"
)

func newListCmd() *cobra.Command {
	var source string
	var categories string
	var showSecrets bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contents of a backup",
		Long:  "Display the contents of a backup folder organized by category.",
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

			fmt.Printf("Backup from: %s (%s)\n", manifest.CreatedAt.Format("2006-01-02 15:04:05"), manifest.Hostname)
			fmt.Printf("macback version: %s\n\n", manifest.MacbackVersion)

			var categoryFilter map[string]bool
			if categories != "" {
				categoryFilter = make(map[string]bool)
				for _, c := range strings.Split(categories, ",") {
					categoryFilter[strings.TrimSpace(c)] = true
				}
			}

			for catName, cat := range manifest.Categories {
				if categoryFilter != nil && !categoryFilter[catName] {
					continue
				}
				if !cat.BackedUp {
					continue
				}

				fmt.Printf("%s (%d files):\n", strings.ToUpper(catName), cat.FileCount)
				for _, f := range cat.Files {
					status := "  "
					if f.Encrypted {
						if showSecrets {
							status = "**"
						} else {
							status = "**"
						}
					}
					fmt.Printf("  %s %s -> %s\n", status, f.Path, f.Original)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&source, "source", "s", "", "backup folder to inspect (required)")
	cmd.Flags().StringVar(&categories, "categories", "", "filter by categories")
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "indicate encrypted files")
	cmd.MarkFlagRequired("source")

	return cmd
}
