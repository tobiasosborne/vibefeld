package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

// newPathCmd creates the path command for showing node ancestry.
func newPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "path <node-id>",
		GroupID: GroupQuery,
		Short:   "Show ancestry chain from root to node",
		Long: `Show the path from the proof root to the specified node.

Displays each ancestor in order with its epistemic state, making it
easy to trace the logical chain leading to any node.

Examples:
  af path 1.6.4.3    Show: 1 → 1.6 → 1.6.4 → 1.6.4.3`,
		Args: cobra.ExactArgs(1),
		RunE: runPath,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")

	return cmd
}

// runPath executes the path command.
func runPath(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")

	nodeID, err := service.ParseNodeID(args[0])
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", args[0], err)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return err
	}

	st, err := svc.LoadState()
	if err != nil {
		return err
	}

	// Build the ancestry chain
	var chain []service.NodeID
	current := nodeID
	chain = append(chain, current)
	for {
		parent, hasParent := current.Parent()
		if !hasParent {
			break
		}
		chain = append([]service.NodeID{parent}, chain...)
		current = parent
	}

	// Render the path
	var ids []string
	for _, id := range chain {
		n := st.GetNode(id)
		if n == nil {
			return fmt.Errorf("node %s not found", id)
		}
		ids = append(ids, fmt.Sprintf("%s [%s]", id.String(), render.ColorEpistemicState(n.EpistemicState)))
	}
	fmt.Fprintln(cmd.OutOrStdout(), strings.Join(ids, " → "))

	// Show the target node's statement
	target := st.GetNode(nodeID)
	if target != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\nStatement: %s\n", target.Statement)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newPathCmd())
}
