package main

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

// newNearbyCmd creates the nearby command for showing local neighborhood.
func newNearbyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "nearby <node-id>",
		GroupID: GroupQuery,
		Short:   "Show parent, siblings, and children of a node",
		Long: `Show the local neighborhood of a node: its parent, siblings, and children.

This provides a focused view of the proof tree around a specific node,
useful for understanding local context without displaying the entire tree.

Examples:
  af nearby 1.6.4    Show parent (1.6), siblings (1.6.*), children (1.6.4.*)`,
		Args: cobra.ExactArgs(1),
		RunE: runNearby,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")

	return cmd
}

// runNearby executes the nearby command.
func runNearby(cmd *cobra.Command, args []string) error {
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

	target := st.GetNode(nodeID)
	if target == nil {
		return fmt.Errorf("node %s not found", nodeID)
	}

	allNodes := st.AllNodes()

	// Find parent
	parentID, hasParent := nodeID.Parent()
	var parentNode *node.Node
	if hasParent {
		parentNode = st.GetNode(parentID)
	}

	// Find siblings (same parent, excluding self)
	var siblings []*node.Node
	if hasParent {
		for _, n := range allNodes {
			p, hp := n.ID.Parent()
			if hp && p.Equal(parentID) && !n.ID.Equal(nodeID) {
				siblings = append(siblings, n)
			}
		}
	}
	sort.Slice(siblings, func(i, j int) bool { return siblings[i].ID.Less(siblings[j].ID) })

	// Find children
	var children []*node.Node
	for _, n := range allNodes {
		p, hp := n.ID.Parent()
		if hp && p.Equal(nodeID) {
			children = append(children, n)
		}
	}
	sort.Slice(children, func(i, j int) bool { return children[i].ID.Less(children[j].ID) })

	// Render
	if parentNode != nil {
		cmd.Printf("Parent:\n  %s\n\n", render.FormatNodeLine(parentNode, st))
	}

	cmd.Printf("Node:\n  %s\n\n", render.FormatNodeLine(target, st))

	if len(siblings) > 0 {
		cmd.Printf("Siblings (%d):\n", len(siblings))
		for _, s := range siblings {
			cmd.Printf("  %s\n", render.FormatNodeLine(s, st))
		}
		cmd.Println()
	}

	if len(children) > 0 {
		cmd.Printf("Children (%d):\n", len(children))
		for _, c := range children {
			cmd.Printf("  %s\n", render.FormatNodeLine(c, st))
		}
	} else {
		cmd.Println("Children: (none — leaf node)")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newNearbyCmd())
}
