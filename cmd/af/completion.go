// Package main provides the af completion command for generating shell completion scripts.
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// newCompletionCmd creates the completion command for generating shell completion scripts.
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for af commands.

This command generates shell-specific completion scripts that enable
tab completion for af commands, flags, and node IDs.

Supported shells: bash, zsh, fish, powershell

INSTALLATION:

Bash:
  # Add to ~/.bashrc:
  source <(af completion bash)

  # Or save to a file:
  af completion bash > /etc/bash_completion.d/af

Zsh:
  # Add to ~/.zshrc (before compinit):
  source <(af completion zsh)

  # Or if using oh-my-zsh, save to completions dir:
  af completion zsh > ~/.oh-my-zsh/completions/_af

Fish:
  # Save to completions directory:
  af completion fish > ~/.config/fish/completions/af.fish

PowerShell:
  # Add to your PowerShell profile:
  af completion powershell | Out-String | Invoke-Expression

FEATURES:

The completion provides:
- Command name completion (e.g., 'af cl<TAB>' completes to 'af claim')
- Flag name completion (e.g., 'af claim --<TAB>' shows available flags)
- Node ID completion (e.g., 'af accept 1.<TAB>' shows child nodes)
- Context-aware completions based on the current proof state`,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MaximumNArgs(1),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// No shell specified, show help
				return cmd.Help()
			}
			return runCompletion(cmd, args[0])
		},
	}

	return cmd
}

// runCompletion generates the completion script for the specified shell.
func runCompletion(cmd *cobra.Command, shell string) error {
	rootCmd := cmd.Root()

	switch strings.ToLower(shell) {
	case "bash":
		return rootCmd.GenBashCompletionV2(cmd.OutOrStdout(), true)
	case "zsh":
		return rootCmd.GenZshCompletion(cmd.OutOrStdout())
	case "fish":
		return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
	case "powershell":
		return rootCmd.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
	default:
		return fmt.Errorf("unsupported shell: %q (supported: bash, zsh, fish, powershell)", shell)
	}
}

// getNodeIDsForCompletion returns all node IDs from the proof in the given directory.
// Returns an empty slice if the directory doesn't contain a valid proof.
func getNodeIDsForCompletion(dir string) []string {
	svc, err := service.NewProofService(dir)
	if err != nil {
		return []string{}
	}

	st, err := svc.LoadState()
	if err != nil {
		return []string{}
	}

	nodes := st.AllNodes()
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID.String())
	}

	return ids
}

// getChallengeIDsForCompletion returns all challenge IDs from the proof in the given directory.
// Returns an empty slice if the directory doesn't contain a valid proof or has no challenges.
func getChallengeIDsForCompletion(dir string) []string {
	svc, err := service.NewProofService(dir)
	if err != nil {
		return []string{}
	}

	st, err := svc.LoadState()
	if err != nil {
		return []string{}
	}

	challenges := st.AllChallenges()
	ids := make([]string, 0, len(challenges))
	for _, c := range challenges {
		ids = append(ids, c.ID)
	}

	return ids
}

// getDefinitionIDsForCompletion returns all definition IDs from the proof in the given directory.
// Returns an empty slice if the directory doesn't contain a valid proof or has no definitions.
// Note: This is a simplified implementation that returns an empty slice.
// Definition IDs are not commonly used in command arguments.
func getDefinitionIDsForCompletion(dir string) []string {
	// Definition completion is not commonly needed and would require
	// scanning the ledger. For now, return empty.
	return []string{}
}

// getExternalIDsForCompletion returns all external reference IDs from the proof.
// Returns an empty slice if the directory doesn't contain a valid proof or has no externals.
// Note: This is a simplified implementation that returns an empty slice.
// External IDs are not commonly used in command arguments.
func getExternalIDsForCompletion(dir string) []string {
	// External completion is not commonly needed and would require
	// filesystem scanning. For now, return empty.
	return []string{}
}

// createNodeIDCompletionFunc creates a Cobra ValidArgsFunction for node ID completion.
// The function reads the --dir flag to determine which proof to load.
func createNodeIDCompletionFunc() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Get the proof directory from --dir flag, default to current directory
		dir, err := cmd.Flags().GetString("dir")
		if err != nil || dir == "" {
			dir = "."
		}

		// Get all node IDs
		nodeIDs := getNodeIDsForCompletion(dir)
		if len(nodeIDs) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Filter by prefix if user has started typing
		if toComplete != "" {
			filtered := make([]string, 0)
			for _, id := range nodeIDs {
				if strings.HasPrefix(id, toComplete) {
					filtered = append(filtered, id)
				}
			}
			return filtered, cobra.ShellCompDirectiveNoFileComp
		}

		return nodeIDs, cobra.ShellCompDirectiveNoFileComp
	}
}

// createChallengeIDCompletionFunc creates a Cobra ValidArgsFunction for challenge ID completion.
func createChallengeIDCompletionFunc() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		dir, err := cmd.Flags().GetString("dir")
		if err != nil || dir == "" {
			dir = "."
		}

		challengeIDs := getChallengeIDsForCompletion(dir)
		if len(challengeIDs) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if toComplete != "" {
			filtered := make([]string, 0)
			for _, id := range challengeIDs {
				if strings.HasPrefix(id, toComplete) {
					filtered = append(filtered, id)
				}
			}
			return filtered, cobra.ShellCompDirectiveNoFileComp
		}

		return challengeIDs, cobra.ShellCompDirectiveNoFileComp
	}
}

// createDefinitionIDCompletionFunc creates a Cobra ValidArgsFunction for definition ID completion.
func createDefinitionIDCompletionFunc() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		dir, err := cmd.Flags().GetString("dir")
		if err != nil || dir == "" {
			dir = "."
		}

		defIDs := getDefinitionIDsForCompletion(dir)
		if len(defIDs) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if toComplete != "" {
			filtered := make([]string, 0)
			for _, id := range defIDs {
				if strings.HasPrefix(id, toComplete) {
					filtered = append(filtered, id)
				}
			}
			return filtered, cobra.ShellCompDirectiveNoFileComp
		}

		return defIDs, cobra.ShellCompDirectiveNoFileComp
	}
}

// createExternalIDCompletionFunc creates a Cobra ValidArgsFunction for external reference ID completion.
func createExternalIDCompletionFunc() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		dir, err := cmd.Flags().GetString("dir")
		if err != nil || dir == "" {
			dir = "."
		}

		extIDs := getExternalIDsForCompletion(dir)
		if len(extIDs) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if toComplete != "" {
			filtered := make([]string, 0)
			for _, id := range extIDs {
				if strings.HasPrefix(id, toComplete) {
					filtered = append(filtered, id)
				}
			}
			return filtered, cobra.ShellCompDirectiveNoFileComp
		}

		return extIDs, cobra.ShellCompDirectiveNoFileComp
	}
}

// RegisterNodeIDCompletion registers node ID completion for a command.
// Call this function in the init() of commands that accept node IDs.
func RegisterNodeIDCompletion(cmd *cobra.Command) {
	cmd.ValidArgsFunction = createNodeIDCompletionFunc()
}

// RegisterChallengeIDCompletion registers challenge ID completion for a command.
func RegisterChallengeIDCompletion(cmd *cobra.Command) {
	cmd.ValidArgsFunction = createChallengeIDCompletionFunc()
}

// RegisterDefinitionIDCompletion registers definition ID completion for a command.
func RegisterDefinitionIDCompletion(cmd *cobra.Command) {
	cmd.ValidArgsFunction = createDefinitionIDCompletionFunc()
}

// RegisterExternalIDCompletion registers external reference ID completion for a command.
func RegisterExternalIDCompletion(cmd *cobra.Command) {
	cmd.ValidArgsFunction = createExternalIDCompletionFunc()
}

func init() {
	rootCmd.AddCommand(newCompletionCmd())

	// Register completion functions for commands that take node IDs
	// This is done after all commands are registered

	// Use a deferred function to ensure all commands exist
	cobra.OnInitialize(func() {
		// Find and register completions for commands that take node IDs
		nodeIDCmds := []string{
			"claim", "refine", "accept", "release", "challenge",
			"get", "deps", "history", "scope", "amend", "archive",
			"admit", "refute",
		}

		challengeIDCmds := []string{
			"resolve-challenge", "withdraw-challenge",
		}

		for _, name := range nodeIDCmds {
			if cmd, _, err := rootCmd.Find([]string{name}); err == nil && cmd != nil {
				RegisterNodeIDCompletion(cmd)
			}
		}

		for _, name := range challengeIDCmds {
			if cmd, _, err := rootCmd.Find([]string{name}); err == nil && cmd != nil {
				RegisterChallengeIDCompletion(cmd)
			}
		}
	})
}
