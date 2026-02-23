package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for macback.

To load completions:

Bash:
  $ source <(macback completion bash)
  # Or add to ~/.bashrc:
  $ macback completion bash > /etc/bash_completion.d/macback

Zsh:
  $ source <(macback completion zsh)
  # Or add to fpath:
  $ macback completion zsh > "${fpath[1]}/_macback"

Fish:
  $ macback completion fish | source
  # Or persist:
  $ macback completion fish > ~/.config/fish/completions/macback.fish`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			}
			return nil
		},
	}
	return cmd
}
