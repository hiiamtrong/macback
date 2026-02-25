package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/hiiamtrong/macback/internal/fsutil"
)

type masApp struct {
	Name string
	ID   string
}

// parseMASLine parses a Brewfile mas entry: mas "App Name", id: 1234567890
// Returns (name, id, true) on success.
func parseMASLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "mas ") {
		return "", "", false
	}
	first := strings.Index(line, `"`)
	if first < 0 {
		return "", "", false
	}
	second := strings.Index(line[first+1:], `"`)
	if second < 0 {
		return "", "", false
	}
	name := line[first+1 : first+1+second]
	_, afterID, found := strings.Cut(line, "id: ")
	if !found {
		return "", "", false
	}
	fields := strings.Fields(strings.TrimSpace(afterID))
	if len(fields) == 0 {
		return "", "", false
	}
	id := strings.TrimRight(fields[0], ",;#")
	return name, id, true
}

// readBrewfileWithoutMAS reads brewfilePath and returns non-mas lines and parsed mas entries.
func readBrewfileWithoutMAS(brewfilePath string) ([]string, []masApp, error) {
	data, err := os.ReadFile(brewfilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading Brewfile: %w", err)
	}
	var nonMas []string
	var apps []masApp
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if name, id, ok := parseMASLine(line); ok {
			apps = append(apps, masApp{Name: name, ID: id})
		} else {
			nonMas = append(nonMas, line)
		}
	}
	return nonMas, apps, scanner.Err()
}

// installMASApps installs each App Store app individually, continuing on failure.
func installMASApps(apps []masApp) {
	if len(apps) == 0 {
		return
	}
	if _, err := exec.LookPath("mas"); err != nil {
		fmt.Println("\nNote: mas CLI not found — App Store apps must be installed manually:")
		for _, app := range apps {
			fmt.Printf("  - %s (id: %s)\n", app.Name, app.ID)
		}
		fmt.Println("Install mas with: brew install mas")
		return
	}
	fmt.Println("\nInstalling App Store apps...")
	for _, app := range apps {
		fmt.Printf("  Installing %s...\n", app.Name)
		c := exec.Command("mas", "install", app.ID)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not install '%s' — install manually from the App Store\n", app.Name)
		}
	}
}

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

			// Restore via Brewfile — mas entries handled separately per-app
			if fsutil.FileExists(brewfilePath) {
				nonMasLines, masApps, err := readBrewfileWithoutMAS(brewfilePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not parse Brewfile: %v\n", err)
					nonMasLines = nil
					masApps = nil
				}

				if len(nonMasLines) > 0 {
					fmt.Println("\nRestoring formulae, casks and taps from Brewfile...")
					tmp, tmpErr := os.CreateTemp("", "macback-Brewfile-*")
					if tmpErr == nil {
						_, _ = fmt.Fprintln(tmp, strings.Join(nonMasLines, "\n"))
						_ = tmp.Close()
						bundleCmd := exec.Command("brew", "bundle", "install", "--file="+tmp.Name())
						bundleCmd.Stdout = os.Stdout
						bundleCmd.Stderr = os.Stderr
						if err := bundleCmd.Run(); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: brew bundle install had errors: %v\n", err)
						}
						_ = os.Remove(tmp.Name())
					} else {
						fmt.Fprintf(os.Stderr, "Warning: could not create temp Brewfile: %v\n", tmpErr)
					}
				}

				installMASApps(masApps)
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
