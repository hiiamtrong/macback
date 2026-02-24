package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/hiiamtrong/macback/internal/config"
	"github.com/hiiamtrong/macback/internal/fsutil"
)

func newInitCmd() *cobra.Command {
	var output string
	var force bool
	var merge bool

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

			if merge {
				// Merge mode: update existing config with new defaults
				existing, err := config.Load(expanded)
				if err != nil {
					return fmt.Errorf("loading existing config: %w (run 'macback init' first)", err)
				}

				defaults := config.DefaultConfig()
				config.MergeDefaults(existing, defaults)

				// Write back
				data, err := yaml.Marshal(existing)
				if err != nil {
					return fmt.Errorf("marshaling config: %w", err)
				}

				header := []byte("# macback configuration file\n# Updated by: macback init --merge\n# Edit this file to customize what gets backed up.\n\n")
				content := append(header, data...)

				if err := os.WriteFile(expanded, content, 0644); err != nil {
					return fmt.Errorf("writing config: %w", err)
				}

				fmt.Printf("Config updated at: %s\n", expanded)
				fmt.Println("New categories added (disabled by default). Edit the file to enable them.")
				return nil
			}

			if !force && fsutil.FileExists(expanded) {
				return fmt.Errorf("config file already exists at %s (use --force to overwrite or --merge to update)", expanded)
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
	cmd.Flags().BoolVar(&merge, "merge", false, "merge new defaults into existing config")

	return cmd
}
