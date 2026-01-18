// Package main contains the af recompute-taint command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/taint"
	"github.com/tobias/vibefeld/internal/types"
)

// TaintChange represents a change in taint state for a node.
type TaintChange struct {
	NodeID   string          `json:"node_id"`
	OldTaint node.TaintState `json:"old_taint"`
	NewTaint node.TaintState `json:"new_taint"`
}

// RecomputeTaintResult represents the result of recomputing taint.
type RecomputeTaintResult struct {
	TotalNodes   int           `json:"total_nodes"`
	NodesChanged int           `json:"nodes_changed"`
	Changes      []TaintChange `json:"changes"`
	DryRun       bool          `json:"dry_run"`
}

// newRecomputeTaintCmd creates the recompute-taint command.
func newRecomputeTaintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "recompute-taint",
		GroupID: GroupAdmin,
		Short:   "Recompute taint state for all nodes",
		Long: `Recompute taint state for all nodes in the proof tree.

Taint propagates through the proof tree based on epistemic states:
- Validated nodes are clean
- Admitted nodes are self_admitted
- Children of self_admitted/tainted nodes become tainted
- Pending nodes are unresolved

Use --dry-run to preview changes without applying them.
Use --verbose for detailed output.

Examples:
  af recompute-taint                    Recompute taint in current directory
  af recompute-taint --dir /path        Recompute in specific directory
  af recompute-taint --dry-run          Preview changes without applying
  af recompute-taint -v                 Verbose output with details
  af recompute-taint -f json            Output in JSON format`,
		RunE: runRecomputeTaint,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().Bool("dry-run", false, "Show what would change without applying")
	cmd.Flags().BoolP("verbose", "v", false, "Verbose output with details")

	return cmd
}

// runRecomputeTaint executes the recompute-taint command.
func runRecomputeTaint(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return err
	}
	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return err
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized by loading state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Get all nodes
	allNodes := st.AllNodes()
	if len(allNodes) == 0 {
		return fmt.Errorf("proof not initialized or empty")
	}

	// Track changes
	var changes []TaintChange
	oldTaints := make(map[string]node.TaintState)

	// Store old taint states
	for _, n := range allNodes {
		oldTaints[n.ID.String()] = n.TaintState
	}

	// Sort nodes by depth (shallower first) to process parents before children
	sortNodesByDepth(allNodes)

	// Build node map for ancestor lookup
	nodeMap := make(map[string]*node.Node)
	for _, n := range allNodes {
		nodeMap[n.ID.String()] = n
	}

	// Recompute taint for each node
	for _, n := range allNodes {
		// Get ancestors
		ancestors := getNodeAncestors(n, nodeMap)

		// Compute new taint
		newTaint := taint.ComputeTaint(n, ancestors)

		// Check if changed
		if n.TaintState != newTaint {
			changes = append(changes, TaintChange{
				NodeID:   n.ID.String(),
				OldTaint: n.TaintState,
				NewTaint: newTaint,
			})
			// Update node in memory (for cascade effect)
			n.TaintState = newTaint
		}
	}

	// Build result
	result := RecomputeTaintResult{
		TotalNodes:   len(allNodes),
		NodesChanged: len(changes),
		Changes:      changes,
		DryRun:       dryRun,
	}

	// If not dry-run, persist changes to ledger
	if !dryRun && len(changes) > 0 {
		if err := persistTaintChanges(dir, changes, st.LatestSeq()); err != nil {
			return fmt.Errorf("error persisting taint changes: %w", err)
		}
	}

	// Output result
	return outputRecomputeTaintResult(cmd, result, verbose, format)
}

// sortNodesByDepth sorts nodes by their depth (shallower first).
func sortNodesByDepth(nodes []*node.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID.Depth() < nodes[j].ID.Depth()
	})
}

// getNodeAncestors returns the ancestor nodes for a given node.
func getNodeAncestors(n *node.Node, nodeMap map[string]*node.Node) []*node.Node {
	var ancestors []*node.Node
	parentID, hasParent := n.ID.Parent()
	for hasParent {
		if parent, ok := nodeMap[parentID.String()]; ok {
			ancestors = append(ancestors, parent)
		}
		parentID, hasParent = parentID.Parent()
	}
	return ancestors
}

// persistTaintChanges writes TaintRecomputed events to the ledger.
func persistTaintChanges(dir string, changes []TaintChange, expectedSeq int) error {
	ledgerDir := filepath.Join(dir, "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		return err
	}

	// Append events for each change
	seq := expectedSeq
	for _, change := range changes {
		// Parse the node ID
		nodeID, err := parseNodeID(change.NodeID)
		if err != nil {
			return fmt.Errorf("invalid node ID %q: %w", change.NodeID, err)
		}

		event := ledger.NewTaintRecomputed(nodeID, change.NewTaint)
		newSeq, err := ldg.AppendIfSequence(event, seq)
		if err != nil {
			return err
		}
		seq = newSeq
	}

	return nil
}

// parseNodeID parses a node ID string and returns a types.NodeID.
func parseNodeID(s string) (types.NodeID, error) {
	return types.Parse(s)
}

// outputRecomputeTaintResult outputs the result based on format.
func outputRecomputeTaintResult(cmd *cobra.Command, result RecomputeTaintResult, verbose bool, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return outputRecomputeTaintJSON(cmd, result)
	default:
		return outputRecomputeTaintText(cmd, result, verbose)
	}
}

// outputRecomputeTaintJSON outputs the result in JSON format.
func outputRecomputeTaintJSON(cmd *cobra.Command, result RecomputeTaintResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputRecomputeTaintText outputs the result in text format.
func outputRecomputeTaintText(cmd *cobra.Command, result RecomputeTaintResult, verbose bool) error {
	if result.DryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "[Dry run] Taint recomputation preview:")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Taint recomputation complete.")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Total nodes: %d\n", result.TotalNodes)
	fmt.Fprintf(cmd.OutOrStdout(), "Nodes changed: %d\n", result.NodesChanged)

	if verbose && len(result.Changes) > 0 {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Changes:")
		for _, change := range result.Changes {
			fmt.Fprintf(cmd.OutOrStdout(), "  Node %s: %s -> %s\n",
				change.NodeID, change.OldTaint, change.NewTaint)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newRecomputeTaintCmd())
}
