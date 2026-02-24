package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trongdev/macos-backup/internal/fsutil"
)

func newRestoreBrewCmd() *cobra.Command {
	var source string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "restore-brew",
		Short: "Restore Homebrew packages from backup",
		Long:  "Restore Homebrew formulae, casks, and taps from a backup folder using the saved Brewfile.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return fmt.Errorf("--source is required")
			}

			expandedSource, err := fsutil.ExpandPath(source)
			if err != nil {
				return fmt.Errorf("expanding source path: %w", err)
			}

			brewDir := filepath.Join(expandedSource, "homebrew")
			if !fsutil.DirExists(brewDir) {
				return fmt.Errorf("no homebrew backup found in %s", expandedSource)
			}

			// Check if brew is installed
			if _, err := exec.LookPath("brew"); err != nil {
				return fmt.Errorf("homebrew is not installed (install from https://brew.sh)")
			}

			// Show what would be restored
			brewfilePath := filepath.Join(brewDir, "Brewfile")
			tapsPath := filepath.Join(brewDir, "brew-taps.txt")
			formulaPath := filepath.Join(brewDir, "brew-list.txt")
			caskPath := filepath.Join(brewDir, "brew-cask-list.txt")

			// Display summary
			fmt.Println("Homebrew restore summary:")
			if fsutil.FileExists(tapsPath) {
				data, _ := os.ReadFile(tapsPath)
				taps := strings.Split(strings.TrimSpace(string(data)), "\n")
				fmt.Printf("  Taps: %d\n", len(taps))
			}
			if fsutil.FileExists(formulaPath) {
				data, _ := os.ReadFile(formulaPath)
				formulas := strings.Split(strings.TrimSpace(string(data)), "\n")
				fmt.Printf("  Formulae: %d\n", len(formulas))
			}
			if fsutil.FileExists(caskPath) {
				data, _ := os.ReadFile(caskPath)
				casks := strings.Split(strings.TrimSpace(string(data)), "\n")
				fmt.Printf("  Casks: %d\n", len(casks))
			}

			if dryRun {
				fmt.Println("\nDry run - no changes made.")
				if fsutil.FileExists(brewfilePath) {
					fmt.Println("\nBrewfile contents:")
					data, _ := os.ReadFile(brewfilePath)
					fmt.Println(string(data))
				}
				return nil
			}

			// Confirm
			fmt.Print("\nProceed with restore? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}

			// Restore taps first
			if fsutil.FileExists(tapsPath) {
				fmt.Println("\nRestoring taps...")
				data, _ := os.ReadFile(tapsPath)
				for _, tap := range strings.Split(strings.TrimSpace(string(data)), "\n") {
					tap = strings.TrimSpace(tap)
					if tap == "" {
						continue
					}
					fmt.Printf("  brew tap %s\n", tap)
					tapCmd := exec.Command("brew", "tap", tap)
					tapCmd.Stdout = os.Stdout
					tapCmd.Stderr = os.Stderr
					_ = tapCmd.Run() // ignore errors, tap may already exist
				}
			}

			// Restore via Brewfile
			if fsutil.FileExists(brewfilePath) {
				fmt.Println("\nRestoring from Brewfile...")
				bundleCmd := exec.Command("brew", "bundle", "install", "--file="+brewfilePath)
				bundleCmd.Stdout = os.Stdout
				bundleCmd.Stderr = os.Stderr
				if err := bundleCmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: brew bundle install had errors: %v\n", err)
				}
			} else {
				fmt.Println("No Brewfile found, skipping bundle install.")
			}

			fmt.Println("\nHomebrew restore complete.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&source, "source", "s", "", "backup source folder (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be restored without doing it")
	_ = cmd.MarkFlagRequired("source")

	return cmd
}
