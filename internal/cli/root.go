package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/hiiamtrong/macback/internal/config"
)

var (
	Version = "dev"
	cfgFile string
	verbose bool
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "macback",
		Short: "macOS config backup and restore tool",
		Long:  "macback backs up your macOS configuration files (SSH, shell, git, dotfiles, Homebrew packages) to a folder with encrypted secrets, and restores them easily.",
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ~/.macback.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(
		newInitCmd(),
		newBackupCmd(),
		newRestoreCmd(),
		newRestoreBrewCmd(),
		newDiffCmd(),
		newListCmd(),
		newVersionCmd(),
		newCompletionCmd(),
	)

	return rootCmd
}

func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	return config.DefaultConfigPath()
}

