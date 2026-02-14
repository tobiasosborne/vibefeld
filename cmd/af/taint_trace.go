package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

func newTaintTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "taint-trace <node-id>",
		GroupID: GroupQuery,
		Short:   "Show taint propagation path for a node",
		Long: `Trace the source of taint for a proof node.

Walks the ancestry chain from the target node to the root, showing each
node's taint state and identifying the exact source of taint propagation.

Taint rules:
  - Archived/refuted nodes are always clean (severed from proof)
  - Pending/draft nodes are unresolved (not yet verified)
  - Admitted nodes are self_admitted (accepted without full proof)
  - Descendants of tainted/self_admitted nodes inherit taint
  - Descendants of unresolved ancestors are unresolved
  - Validated nodes with clean ancestry are clean

Examples:
  af taint-trace 1.6.4      Show why node 1.6.4 is tainted
  af taint-trace 1.2 -f json  JSON output for machine consumption`,
		Args: cobra.ExactArgs(1),
		RunE: runTaintTrace,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

// traceEntry represents one node in the taint trace.
type traceEntry struct {
	NodeID         string `json:"node_id"`
	EpistemicState string `json:"epistemic_state"`
	TaintState     string `json:"taint_state"`
	Reason         string `json:"reason"`
	IsSource       bool   `json:"is_source"`
}

func runTaintTrace(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af taint-trace")
		return render.InvalidNodeIDError("af taint-trace", nodeIDStr, examples)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	target := st.GetNode(nodeID)
	if target == nil {
		return fmt.Errorf("node %s not found", nodeID.String())
	}

	// Build ancestry chain (root first, target last)
	var chain []*node.Node
	current := nodeID
	for {
		n := st.GetNode(current)
		if n == nil {
			break
		}
		chain = append([]*node.Node{n}, chain...)
		parent, hasParent := current.Parent()
		if !hasParent {
			break
		}
		current = parent
	}

	// Compute trace: for each node in the chain, determine its taint reason
	entries := make([]traceEntry, len(chain))
	for i, n := range chain {
		entry := traceEntry{
			NodeID:         n.ID.String(),
			EpistemicState: string(n.EpistemicState),
			TaintState:     string(n.TaintState),
		}

		entry.Reason, entry.IsSource = taintReason(n, chain[:i])
		entries[i] = entry
	}

	switch strings.ToLower(format) {
	case "json":
		return outputTaintTraceJSON(cmd, target, entries)
	default:
		return outputTaintTraceText(cmd, target, entries)
	}
}

// taintReason determines why a node has its current taint state.
// ancestors is the ancestry chain BEFORE this node (root to parent).
func taintReason(n *node.Node, ancestors []*node.Node) (reason string, isSource bool) {
	// Rule 0: Archived/refuted
	if n.EpistemicState == schema.EpistemicArchived {
		return "archived — severed from proof tree", false
	}
	if n.EpistemicState == schema.EpistemicRefuted {
		return "refuted — severed from proof tree", false
	}

	// Rule 1: Pending/draft → unresolved (self-caused)
	if n.EpistemicState == schema.EpistemicPending {
		return "node is pending verification", true
	}
	if n.EpistemicState == schema.EpistemicDraft {
		return "node is in draft state", true
	}

	// Rule 2: Ancestor unresolved → propagated
	for _, a := range ancestors {
		if a.TaintState == node.TaintUnresolved {
			return fmt.Sprintf("ancestor %s is unresolved", a.ID.String()), false
		}
	}

	// Rule 3: Self-admitted
	if schema.IntroducesTaint(n.EpistemicState) {
		return fmt.Sprintf("node is %s — accepted without full proof", n.EpistemicState), true
	}

	// Rule 4: Ancestor tainted/self_admitted
	for _, a := range ancestors {
		if a.TaintState == node.TaintTainted || a.TaintState == node.TaintSelfAdmitted {
			return fmt.Sprintf("ancestor %s is %s", a.ID.String(), a.TaintState), false
		}
	}

	// Rule 5: Clean
	return "all ancestors verified, no taint sources", false
}

func outputTaintTraceJSON(cmd *cobra.Command, target *node.Node, entries []traceEntry) error {
	result := map[string]interface{}{
		"node_id":     target.ID.String(),
		"taint_state": string(target.TaintState),
		"trace":       entries,
	}
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}

func outputTaintTraceText(cmd *cobra.Command, target *node.Node, entries []traceEntry) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Taint trace for node %s\n", target.ID.String())
	fmt.Fprintf(w, "Current taint: %s\n\n", render.ColorTaintState(target.TaintState))

	if target.TaintState == node.TaintClean {
		fmt.Fprintln(w, "This node is clean — no taint in ancestry chain.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Ancestry:")
		for _, e := range entries {
			fmt.Fprintf(w, "  %s [%s] %s\n", e.NodeID,
				render.ColorEpistemicState(schema.EpistemicState(e.EpistemicState)),
				render.ColorTaintState(node.TaintState(e.TaintState)))
		}
		return nil
	}

	// Find taint sources
	var sources []traceEntry
	for _, e := range entries {
		if e.IsSource {
			sources = append(sources, e)
		}
	}

	if len(sources) > 0 {
		fmt.Fprintln(w, "Taint source(s):")
		for _, s := range sources {
			fmt.Fprintf(w, "  %s — %s\n", s.NodeID, s.Reason)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Full trace (root to target):")
	for _, e := range entries {
		marker := "  "
		if e.IsSource {
			marker = "> "
		}
		fmt.Fprintf(w, "%s%s [%s] %s",
			marker,
			e.NodeID,
			render.ColorEpistemicState(schema.EpistemicState(e.EpistemicState)),
			render.ColorTaintState(node.TaintState(e.TaintState)))
		if e.Reason != "" {
			fmt.Fprintf(w, " — %s", e.Reason)
		}
		fmt.Fprintln(w)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newTaintTraceCmd())
}
