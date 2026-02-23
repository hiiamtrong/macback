package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/trongdev/macos-backup/internal/config"
	"github.com/trongdev/macos-backup/internal/fsutil"
)

func newInitCmd() *cobra.Command {
	var output string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate default config file",
		Long:  "Generate a default macback configuration file at ~/.macback.yaml (or a custom path).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if output == "" {
				output = config.DefaultConfigPath()
			}

			expanded, err := fsutil.ExpandPath(output)
			if err != nil {
				return fmt.Errorf("expanding path: %w", err)
			}

			if !force && fsutil.FileExists(expanded) {
				return fmt.Errorf("config file already exists at %s (use --force to overwrite)", expanded)
			}

			if err := config.WriteDefault(output); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			fmt.Printf("Config file created at: %s\n", expanded)
			fmt.Println("Edit this file to customize what gets backed up.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "output path (default: ~/.macback.yaml)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config file")

	return cmd
}
