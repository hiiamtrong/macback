package cli

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hiiamtrong/macback/internal/backup"
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

// zshPluginDef describes a known third-party Oh My Zsh plugin that must be
// cloned into $ZSH_CUSTOM/plugins/ (built-in plugins need no extra install).
type zshPluginDef struct {
	Name     string // plugin directory name and how it appears in plugins=(...)
	CloneURL string // git repository to clone
}

// gitProjectDef describes a git repository found in the projects backup.
type gitProjectDef struct {
	Name      string // display name, e.g. "Personal/hayhaytv"
	CloneURL  string // remote origin URL
	CloneDest string // contracted original path, e.g. "~/Works/Personal/hayhaytv"
}

// knownZshPlugins lists popular third-party Oh My Zsh plugins in the order
// they should be installed.
var knownZshPlugins = []zshPluginDef{
	{"zsh-autosuggestions", "https://github.com/zsh-users/zsh-autosuggestions"},
	{"zsh-syntax-highlighting", "https://github.com/zsh-users/zsh-syntax-highlighting"},
	{"fast-syntax-highlighting", "https://github.com/zdharma-continuum/fast-syntax-highlighting"},
	{"zsh-completions", "https://github.com/zsh-users/zsh-completions"},
	{"zsh-history-substring-search", "https://github.com/zsh-users/zsh-history-substring-search"},
	{"zsh-autocomplete", "https://github.com/marlonrichert/zsh-autocomplete"},
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

	manifest, _ := backup.ReadManifest(backupDir)

	shellDir := filepath.Join(backupDir, "shell")
	detected := detectToolsInShellFiles(shellDir)
	plugins := detectZshPlugins(shellDir)
	gitProjects := detectGitProjects(backupDir, manifest)
	scriptBrewfile := copyBrewfileForScript(filepath.Join(backupDir, "homebrew", "Brewfile"), outputPath)

	script := generateBootstrapScript(detected, plugins, gitProjects, scriptBrewfile)
	if err := os.WriteFile(outputPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("writing setup script: %w", err)
	}

	printBootstrapSummary(detected, plugins, gitProjects, scriptBrewfile, outputPath)

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

// printBootstrapSummary prints the list of tools and plugins in the setup script.
func printBootstrapSummary(detected map[string]bool, plugins []zshPluginDef, gitProjects []gitProjectDef, scriptBrewfile, outputPath string) {
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
		fmt.Printf("  %-30s [%s]\n", tool.Name, status)
	}
	for _, p := range plugins {
		status := "not installed"
		if isZshPluginInstalled(p) {
			status = "already installed"
		}
		fmt.Printf("  %-30s [%s]\n", "zsh: "+p.Name, status)
	}
	if scriptBrewfile != "" {
		fmt.Printf("  %-30s [will run brew bundle]\n", "Homebrew packages")
	}
	for _, p := range gitProjects {
		status := "will clone"
		if expanded, err := fsutil.ExpandPath(p.CloneDest); err == nil && fsutil.DirExists(expanded) {
			status = "already exists"
		}
		fmt.Printf("  %-30s [%s]\n", "git: "+p.Name, status)
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

// detectZshPlugins reads shell config files in shellDir and returns the subset
// of knownZshPlugins whose names appear in the file contents.
func detectZshPlugins(shellDir string) []zshPluginDef {
	entries, err := os.ReadDir(shellDir)
	if err != nil {
		return nil
	}

	found := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(shellDir, e.Name()))
		if err != nil {
			continue
		}
		text := string(content)
		for _, p := range knownZshPlugins {
			if strings.Contains(text, p.Name) {
				found[p.Name] = true
			}
		}
	}

	var result []zshPluginDef
	for _, p := range knownZshPlugins {
		if found[p.Name] {
			result = append(result, p)
		}
	}
	return result
}

// isZshPluginInstalled reports whether the plugin directory already exists
// under $HOME/.oh-my-zsh/custom/plugins/.
func isZshPluginInstalled(p zshPluginDef) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return fsutil.DirExists(filepath.Join(home, ".oh-my-zsh", "custom", "plugins", p.Name))
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
func generateBootstrapScript(detected map[string]bool, plugins []zshPluginDef, gitProjects []gitProjectDef, brewfilePath string) string {
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

	if len(plugins) > 0 {
		writeZshPluginsBlock(&sb, plugins)
	}

	if brewfilePath != "" {
		writeBrewfileBlock(&sb, brewfilePath)
	}

	if len(gitProjects) > 0 {
		writeGitProjectsBlock(&sb, gitProjects)
	}

	sb.WriteString("echo \"\"\n")
	sb.WriteString("echo \"=== Setup complete! ===\"\n")
	sb.WriteString("echo \"Please restart your terminal or run: source ~/.zshrc\"\n")

	return sb.String()
}

// writeZshPluginsBlock appends idempotent git-clone blocks for each Oh My Zsh plugin.
func writeZshPluginsBlock(sb *strings.Builder, plugins []zshPluginDef) {
	sb.WriteString("# --- Oh My Zsh plugins ---\n")
	sb.WriteString("if [ -d \"$HOME/.oh-my-zsh\" ]; then\n")
	sb.WriteString("  ZSH_CUSTOM=\"${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}\"\n")
	for _, p := range plugins {
		fmt.Fprintf(sb, "  if [ ! -d \"$ZSH_CUSTOM/plugins/%s\" ]; then\n", p.Name)
		fmt.Fprintf(sb, "    echo \">>> Installing %s...\"\n", p.Name)
		fmt.Fprintf(sb, "    git clone %s \"$ZSH_CUSTOM/plugins/%s\"\n", p.CloneURL, p.Name)
		sb.WriteString("  fi\n")
	}
	sb.WriteString("fi\n\n")
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
// mas (App Store) entries are handled separately per-app so one failure does not block others.
func writeBrewfileBlock(sb *strings.Builder, brewfilePath string) {
	sb.WriteString("# --- Homebrew packages (Brewfile) ---\n")
	fmt.Fprintf(sb, "if command -v brew &>/dev/null && [ -f \"%s\" ]; then\n", brewfilePath)
	// Run brew bundle with mas entries filtered out (formulae, casks, taps only)
	sb.WriteString("  echo \">>> Installing Homebrew packages...\"\n")
	sb.WriteString("  _macback_tmp=$(mktemp)\n")
	fmt.Fprintf(sb, "  grep -v '^mas ' \"%s\" > \"$_macback_tmp\"\n", brewfilePath)
	sb.WriteString("  brew bundle --file=\"$_macback_tmp\" || echo \"Warning: some packages failed to install — check output above\"\n")
	sb.WriteString("  rm -f \"$_macback_tmp\"\n")
	// Install App Store apps one-by-one so a single failure does not block others
	fmt.Fprintf(sb, "  if grep -q '^mas ' \"%s\" 2>/dev/null; then\n", brewfilePath)
	sb.WriteString("    if command -v mas &>/dev/null; then\n")
	sb.WriteString("      echo \">>> Installing App Store apps...\"\n")
	fmt.Fprintf(sb, "      while IFS= read -r _masline; do\n")
	sb.WriteString("        _masid=$(echo \"$_masline\" | grep -oE 'id: [0-9]+' | grep -oE '[0-9]+')\n")
	sb.WriteString("        _masname=$(echo \"$_masline\" | sed -E 's/mas \"([^\"]+)\".*/\\1/')\n")
	sb.WriteString("        [ -z \"$_masid\" ] && continue\n")
	sb.WriteString("        echo \"  Installing $_masname...\"\n")
	sb.WriteString("        mas install \"$_masid\" || echo \"  Warning: could not install '$_masname' — install manually from the App Store\"\n")
	fmt.Fprintf(sb, "      done < <(grep '^mas ' \"%s\")\n", brewfilePath)
	sb.WriteString("    else\n")
	sb.WriteString("      echo \"  Note: mas CLI not found — install with: brew install mas\"\n")
	sb.WriteString("    fi\n")
	sb.WriteString("  fi\n")
	sb.WriteString("fi\n\n")
}

// writeGitProjectsBlock appends idempotent git clone blocks for each detected project.
func writeGitProjectsBlock(sb *strings.Builder, projects []gitProjectDef) {
	sb.WriteString("# --- Git repositories ---\n")
	sb.WriteString("if command -v git &>/dev/null; then\n")
	sb.WriteString("  echo \">>> Cloning git repositories...\"\n")
	for _, p := range projects {
		dest := homeToShellVar(p.CloneDest)
		fmt.Fprintf(sb, "  if [ ! -d \"%s\" ]; then\n", dest)
		fmt.Fprintf(sb, "    echo \"  Cloning %s...\"\n", p.Name)
		fmt.Fprintf(sb, "    mkdir -p \"%s\"\n", filepath.Dir(dest))
		fmt.Fprintf(sb, "    git clone %s \"%s\" || echo \"  Warning: could not clone %s\"\n", p.CloneURL, dest, p.Name)
		sb.WriteString("  fi\n")
	}
	sb.WriteString("fi\n\n")
}

// detectGitProjects scans the backup's projects directory for git repositories
// and returns those that have a remote origin URL and a known original path.
func detectGitProjects(backupDir string, manifest *backup.Manifest) []gitProjectDef {
	projectsDir := filepath.Join(backupDir, "projects")
	if !fsutil.DirExists(projectsDir) {
		return nil
	}
	cat := projectsManifestCategory(manifest)
	var result []gitProjectDef
	seen := make(map[string]bool)
	_ = filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Name() != "config" || filepath.Base(filepath.Dir(path)) != ".git" {
			return nil
		}
		projectRoot := filepath.Dir(filepath.Dir(path))
		if seen[projectRoot] {
			return nil
		}
		seen[projectRoot] = true
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		remoteURL := parseGitRemoteURL(string(content))
		if remoteURL == "" {
			return nil
		}
		relProjectRoot, _ := filepath.Rel(backupDir, projectRoot)
		originalDir := findProjectOriginalDir(relProjectRoot, cat)
		if originalDir == "" {
			return nil
		}
		displayName, _ := filepath.Rel(projectsDir, projectRoot)
		result = append(result, gitProjectDef{
			Name:      displayName,
			CloneURL:  remoteURL,
			CloneDest: originalDir,
		})
		return nil
	})
	return result
}

// projectsManifestCategory returns the projects ManifestCategory, or nil if absent.
func projectsManifestCategory(manifest *backup.Manifest) *backup.ManifestCategory {
	if manifest == nil {
		return nil
	}
	cat, ok := manifest.Categories["projects"]
	if !ok || !cat.BackedUp {
		return nil
	}
	return cat
}

// findProjectOriginalDir returns the original directory for a project by looking
// up manifest entries whose backup path starts with relProjectRoot.
func findProjectOriginalDir(relProjectRoot string, cat *backup.ManifestCategory) string {
	if cat == nil {
		return ""
	}
	for _, f := range cat.Files {
		if !strings.HasPrefix(f.Path, relProjectRoot+"/") {
			continue
		}
		suffix := strings.TrimPrefix(f.Path, relProjectRoot+"/")
		if orig, ok := strings.CutSuffix(f.Original, "/"+suffix); ok {
			return orig
		}
	}
	return ""
}

// parseGitRemoteURL extracts the remote origin URL from git config file content.
func parseGitRemoteURL(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	inOrigin := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == `[remote "origin"]` {
			inOrigin = true
			continue
		}
		if strings.HasPrefix(line, "[") && inOrigin {
			break
		}
		if inOrigin && strings.HasPrefix(line, "url = ") {
			return strings.TrimPrefix(line, "url = ")
		}
	}
	return ""
}

// homeToShellVar replaces a leading ~ with $HOME for use inside shell scripts.
func homeToShellVar(path string) string {
	if path == "~" {
		return "$HOME"
	}
	if strings.HasPrefix(path, "~/") {
		return "$HOME/" + path[2:]
	}
	return path
}
