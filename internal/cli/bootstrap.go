package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hiiamtrong/macback/internal/fsutil"
	"github.com/spf13/cobra"
)

// toolDef describes a known developer tool: how to detect it and how to install it.
type toolDef struct {
	Name        string
	BinCheck    string   // executable name to check with `command -v`; empty means use DirCheck
	DirCheck    string   // directory to check for existence (relative to $HOME)
	Patterns    []string // substrings in shell config files that indicate this tool is used
	InstallCmds []string // bash lines that install the tool
}

// bootstrapTools lists developer tools in installation order.
// Homebrew must be first because rbenv and pyenv install via brew.
var bootstrapTools = []toolDef{
	{
		Name:        "Homebrew",
		BinCheck:    "brew",
		Patterns:    nil, // always included — everything else may depend on it
		InstallCmds: []string{
			`echo ">>> Installing Homebrew..."`,
			`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`,
			`# Apple Silicon: add brew to PATH for this session`,
			`if [ -f /opt/homebrew/bin/brew ]; then eval "$(/opt/homebrew/bin/brew shellenv)"; fi`,
		},
	},
	{
		Name:     "Oh My Zsh",
		DirCheck: ".oh-my-zsh",
		Patterns: []string{".oh-my-zsh", "ZSH_THEME", "oh-my-zsh"},
		InstallCmds: []string{
			`echo ">>> Installing Oh My Zsh..."`,
			`sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended`,
		},
	},
	{
		Name:     "Rust",
		BinCheck: "cargo",
		Patterns: []string{".cargo", "rustup", "cargo"},
		InstallCmds: []string{
			`echo ">>> Installing Rust..."`,
			`curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y`,
			`source "$HOME/.cargo/env"`,
		},
	},
	{
		Name:     "rbenv",
		BinCheck: "rbenv",
		Patterns: []string{"rbenv"},
		InstallCmds: []string{
			`echo ">>> Installing rbenv..."`,
			`brew install rbenv ruby-build`,
		},
	},
	{
		Name:     "nvm",
		DirCheck: ".nvm",
		Patterns: []string{"nvm", "NVM_DIR"},
		InstallCmds: []string{
			`echo ">>> Installing nvm..."`,
			`curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash`,
		},
	},
	{
		Name:     "pyenv",
		BinCheck: "pyenv",
		Patterns: []string{"pyenv"},
		InstallCmds: []string{
			`echo ">>> Installing pyenv..."`,
			`brew install pyenv`,
		},
	},
}

func newBootstrapCmd() *cobra.Command {
	var source string
	var outputPath string
	var run bool

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Generate a setup script from a backup",
		Long: `Scan a backup and generate a shell script that installs missing developer
tools (Homebrew, Oh My Zsh, Rust, rbenv, nvm, pyenv) and Homebrew packages.

The script is written to ~/macback-setup.sh by default. Run it on a fresh
machine after restoring your files to get your environment back up quickly.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBootstrap(source, outputPath, run)
		},
	}

	cmd.Flags().StringVarP(&source, "source", "s", "", "backup source folder or .zip archive (required)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output path for setup script (default: ~/macback-setup.sh)")
	cmd.Flags().BoolVar(&run, "run", false, "run the generated script immediately after generating it")
	_ = cmd.MarkFlagRequired("source")

	return cmd
}

// runBootstrap implements the bootstrap command logic.
func runBootstrap(source, outputPath string, run bool) error {
	backupDir, cleanup, err := resolveBackupSource(source)
	if err != nil {
		return err
	}
	defer cleanup()

	outputPath, err = resolveOutputPath(outputPath)
	if err != nil {
		return err
	}

	detected := detectToolsInShellFiles(filepath.Join(backupDir, "shell"))
	scriptBrewfile := copyBrewfileForScript(filepath.Join(backupDir, "homebrew", "Brewfile"), outputPath)

	script := generateBootstrapScript(detected, scriptBrewfile)
	if err := os.WriteFile(outputPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("writing setup script: %w", err)
	}

	printBootstrapSummary(detected, scriptBrewfile, outputPath)

	if run {
		return execBootstrapScript(outputPath)
	}
	return nil
}

// resolveOutputPath returns the absolute output path, defaulting to ~/macback-setup.sh.
func resolveOutputPath(outputPath string) (string, error) {
	if outputPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		return filepath.Join(home, "macback-setup.sh"), nil
	}
	expanded, err := fsutil.ExpandPath(outputPath)
	if err != nil {
		return "", fmt.Errorf("expanding output path: %w", err)
	}
	return expanded, nil
}

// copyBrewfileForScript copies the Brewfile from the backup alongside the output
// script so the generated script references a stable path even if the source was
// a .zip archive that gets cleaned up.  Returns the stable path, or "" on failure.
func copyBrewfileForScript(brewfilePath, outputPath string) string {
	if !fsutil.FileExists(brewfilePath) {
		return ""
	}
	dest := filepath.Join(filepath.Dir(outputPath), "macback-setup-Brewfile")
	if err := fsutil.CopyFile(brewfilePath, dest); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not copy Brewfile: %v\n", err)
		return ""
	}
	return dest
}

// printBootstrapSummary prints the list of tools included in the setup script.
func printBootstrapSummary(detected map[string]bool, scriptBrewfile, outputPath string) {
	fmt.Printf("Setup script written to: %s\n", outputPath)
	if scriptBrewfile != "" {
		fmt.Printf("Brewfile copied to:      %s\n", scriptBrewfile)
	}
	fmt.Println()
	fmt.Println("Tools included in setup script:")
	for _, tool := range bootstrapTools {
		if tool.Patterns != nil && !detected[tool.Name] {
			continue
		}
		status := "not installed"
		if isToolInstalled(tool) {
			status = "already installed"
		}
		fmt.Printf("  %-20s [%s]\n", tool.Name, status)
	}
	if scriptBrewfile != "" {
		fmt.Printf("  %-20s [will run brew bundle]\n", "Homebrew packages")
	}
	fmt.Printf("\nTo run the setup:\n  bash %s\n", outputPath)
}

// execBootstrapScript runs the generated script via bash.
func execBootstrapScript(outputPath string) error {
	fmt.Println("\nRunning setup script...")
	c := exec.Command("bash", outputPath)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if err := c.Run(); err != nil {
		return fmt.Errorf("setup script failed: %w", err)
	}
	return nil
}

// detectToolsInShellFiles reads all shell config files in shellDir and returns
// the set of tool names whose patterns were found.
func detectToolsInShellFiles(shellDir string) map[string]bool {
	detected := make(map[string]bool)

	entries, err := os.ReadDir(shellDir)
	if err != nil {
		return detected // shell dir missing or unreadable — that's fine
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(shellDir, e.Name()))
		if err != nil {
			continue
		}
		scanContentForTools(string(content), detected)
	}

	return detected
}

// scanContentForTools checks text for tool patterns and marks matches in detected.
func scanContentForTools(text string, detected map[string]bool) {
	for _, tool := range bootstrapTools {
		if tool.Patterns == nil || detected[tool.Name] {
			continue
		}
		if textContainsAny(text, tool.Patterns) {
			detected[tool.Name] = true
		}
	}
}

// textContainsAny reports whether text contains any of the given substrings.
func textContainsAny(text string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

// isToolInstalled reports whether the tool is currently installed on this machine.
func isToolInstalled(tool toolDef) bool {
	if tool.BinCheck != "" {
		_, err := exec.LookPath(tool.BinCheck)
		return err == nil
	}
	if tool.DirCheck != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		return fsutil.DirExists(filepath.Join(home, tool.DirCheck))
	}
	return false
}

// toolBashCondition returns the bash condition (for use in `if <cond>; then`)
// that is true when the tool is NOT yet installed.
func toolBashCondition(tool toolDef) string {
	if tool.BinCheck != "" {
		return fmt.Sprintf("! command -v %s &>/dev/null", tool.BinCheck)
	}
	return fmt.Sprintf(`[ ! -d "$HOME/%s" ]`, tool.DirCheck)
}

// generateBootstrapScript returns the full content of the setup shell script.
func generateBootstrapScript(detected map[string]bool, brewfilePath string) string {
	var sb strings.Builder

	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# macback bootstrap setup script\n")
	sb.WriteString("# Generated by: macback bootstrap\n")
	sb.WriteString("# Run this on a fresh machine after restoring your backup.\n\n")
	sb.WriteString("echo \"=== macback Bootstrap Setup ===\"\n")
	sb.WriteString("echo \"\"\n\n")

	for _, tool := range bootstrapTools {
		if tool.Patterns != nil && !detected[tool.Name] {
			continue
		}
		writeToolBlock(&sb, tool)
	}

	if brewfilePath != "" {
		writeBrewfileBlock(&sb, brewfilePath)
	}

	sb.WriteString("echo \"\"\n")
	sb.WriteString("echo \"=== Setup complete! ===\"\n")
	sb.WriteString("echo \"Please restart your terminal or run: source ~/.zshrc\"\n")

	return sb.String()
}

// writeToolBlock appends an idempotent install block for a single tool to sb.
func writeToolBlock(sb *strings.Builder, tool toolDef) {
	fmt.Fprintf(sb, "# --- %s ---\n", tool.Name)
	fmt.Fprintf(sb, "if %s; then\n", toolBashCondition(tool))
	for _, line := range tool.InstallCmds {
		sb.WriteString("  " + line + "\n")
	}
	sb.WriteString("fi\n\n")
}

// writeBrewfileBlock appends the brew bundle install block to sb.
func writeBrewfileBlock(sb *strings.Builder, brewfilePath string) {
	sb.WriteString("# --- Homebrew packages (Brewfile) ---\n")
	fmt.Fprintf(sb, "if command -v brew &>/dev/null && [ -f \"%s\" ]; then\n", brewfilePath)
	sb.WriteString("  echo \">>> Installing Homebrew packages...\"\n")
	fmt.Fprintf(sb, "  brew bundle --file=\"%s\" || echo \"Warning: some packages failed to install — check output above\"\n", brewfilePath)
	sb.WriteString("fi\n\n")
}
