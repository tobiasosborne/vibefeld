// Package main contains the af deps command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/service"
)

// newDepsCmd creates the deps command for showing dependency graph.
func newDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deps <node-id>",
		GroupID: GroupQuery,
		Short:   "Show dependency graph for a node",
		Long: `Show the dependency graph for a proof node.

This displays both reference dependencies (nodes this node cites) and
validation dependencies (nodes that must be validated before this node
can be accepted).

For each dependency, shows:
- Node ID
- Statement (truncated)
- Epistemic state (pending, validated, admitted, etc.)
- Whether it's blocking acceptance

Examples:
  af deps 1.3              Show dependencies for node 1.3
  af deps 1.3 -f json      Output as JSON
  af deps 1.3 -d ./proof   Use specific proof directory`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeps(cmd, args[0])
		},
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")

	return cmd
}

func runDeps(cmd *cobra.Command, nodeIDStr string) error {
	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	// Parse node ID
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", nodeIDStr, err)
	}

	// Create service and load state
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to load proof: %w", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Get the target node
	node := st.GetNode(nodeID)
	if node == nil {
		return fmt.Errorf("node %s not found\n\nHint: Use 'af status' to see all available nodes.", nodeIDStr)
	}

	// Build dependency info
	type depInfo struct {
		ID         string `json:"id"`
		Statement  string `json:"statement"`
		State      string `json:"epistemic_state"`
		IsBlocking bool   `json:"is_blocking,omitempty"`
		DepType    string `json:"type"` // "reference" or "validation"
		Amended    bool   `json:"amended,omitempty"`
	}

	// Compute the per-kind edges an amendment added or removed. An edge that is
	// currently present is marked with `*` only when an amendment ADDED it; an
	// edge that an amendment removed is reported separately, so removing one
	// edge no longer marks every surviving edge.
	amended := computeAmendedEdgeSummary(st.GetAmendmentHistory(nodeID))

	var deps []depInfo

	// Process reference dependencies
	for _, depID := range node.Dependencies {
		dep := st.GetNode(depID)
		info := depInfo{
			ID:      depID.String(),
			DepType: "reference",
			Amended: amended.refAdded[depID.String()],
		}
		if dep != nil {
			info.Statement = truncateString(dep.Statement, 50)
			info.State = string(dep.EpistemicState)
		} else {
			info.Statement = "(not found)"
			info.State = "unknown"
		}
		deps = append(deps, info)
	}

	// Process validation dependencies
	for _, depID := range node.ValidationDeps {
		dep := st.GetNode(depID)
		info := depInfo{
			ID:      depID.String(),
			DepType: "validation",
			Amended: amended.valAdded[depID.String()],
		}
		if dep != nil {
			info.Statement = truncateString(dep.Statement, 50)
			info.State = string(dep.EpistemicState)
			// A validation dep is blocking if not validated/admitted
			if dep.EpistemicState != service.EpistemicValidated && dep.EpistemicState != service.EpistemicAdmitted {
				info.IsBlocking = true
			}
		} else {
			info.Statement = "(not found)"
			info.State = "unknown"
			info.IsBlocking = true
		}
		deps = append(deps, info)
	}

	// Count blocking deps
	blockingCount := 0
	for _, d := range deps {
		if d.IsBlocking {
			blockingCount++
		}
	}

	// Output
	if format == "json" {
		result := map[string]interface{}{
			"node_id":         nodeIDStr,
			"statement":       node.Statement,
			"epistemic_state": string(node.EpistemicState),
			"dependencies":    deps,
			"blocking_count":  blockingCount,
		}
		if len(node.ValidationDeps) > 0 {
			result["validation_deps"] = service.ToStringSlice(node.ValidationDeps)
		}
		if len(node.Dependencies) > 0 {
			result["reference_deps"] = service.ToStringSlice(node.Dependencies)
		}
		if removed := amended.removedLists(); len(removed) > 0 {
			result["removed_by_amendment"] = removed
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
	} else {
		// Text output
		fmt.Fprintf(cmd.OutOrStdout(), "Dependencies for node %s\n", nodeIDStr)
		fmt.Fprintf(cmd.OutOrStdout(), "Statement: %s\n", truncateString(node.Statement, 60))
		fmt.Fprintf(cmd.OutOrStdout(), "State: %s\n\n", node.EpistemicState)

		if len(deps) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No dependencies")
		} else {
			// Group by type
			var refDeps, valDeps []depInfo
			for _, d := range deps {
				if d.DepType == "reference" {
					refDeps = append(refDeps, d)
				} else {
					valDeps = append(valDeps, d)
				}
			}

			if len(refDeps) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Reference Dependencies:")
				for _, d := range refDeps {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s%s [%s] - %s\n", amendedMark(d.Amended), d.ID, d.State, d.Statement)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			if len(valDeps) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Validation Dependencies (must be validated before accepting):")
				for _, d := range valDeps {
					status := d.State
					if d.IsBlocking {
						status += " (BLOCKING)"
					} else {
						status += " (satisfied)"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s%s [%s] - %s\n", amendedMark(d.Amended), d.ID, status, d.Statement)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			if len(amended.refAdded) > 0 || len(amended.valAdded) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(*) edge added by a dependency amendment (af amend-deps)")
			}
			if removed := amended.removedLists(); len(removed) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "(-) removed by amendment: %s\n", strings.Join(removed, ", "))
			}

			if blockingCount > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Status: BLOCKED - %d validation dependencies are unvalidated\n", blockingCount)
				fmt.Fprintln(cmd.OutOrStdout(), "Cannot accept this node until all validation dependencies are validated.")
			} else if len(valDeps) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Status: Ready - all validation dependencies are satisfied")
			}
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newDepsCmd())
}

// amendedEdgeSummary accumulates, per edge kind, the edges added and removed by
// this node's dependency amendments, applied in ledger order so a later re-add
// cancels an earlier removal and vice versa.
type amendedEdgeSummary struct {
	refAdded, refRemoved map[string]bool
	valAdded, valRemoved map[string]bool
}

// computeAmendedEdgeSummary computes the add/remove deltas of every dependency
// amendment on the node.
func computeAmendedEdgeSummary(amendments []service.Amendment) amendedEdgeSummary {
	s := amendedEdgeSummary{
		refAdded:   map[string]bool{},
		refRemoved: map[string]bool{},
		valAdded:   map[string]bool{},
		valRemoved: map[string]bool{},
	}
	for _, a := range amendments {
		if a.Kind != service.AmendmentKindDependencies {
			continue
		}
		applyEdgeDelta(s.refAdded, s.refRemoved, a.PreviousDependencies, a.NewDependencies)
		applyEdgeDelta(s.valAdded, s.valRemoved, a.PreviousValidationDeps, a.NewValidationDeps)
	}
	return s
}

// applyEdgeDelta folds one amendment's edge replacement into the running
// added/removed sets: new-minus-previous is added, previous-minus-new is
// removed, and an edge moving the other way cancels its prior entry.
func applyEdgeDelta(added, removed map[string]bool, previous, next []service.NodeID) {
	prevSet := idSet(previous)
	nextSet := idSet(next)
	for id := range nextSet {
		if !prevSet[id] {
			added[id] = true
			delete(removed, id)
		}
	}
	for id := range prevSet {
		if !nextSet[id] {
			removed[id] = true
			delete(added, id)
		}
	}
}

func idSet(ids []service.NodeID) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id.String()] = true
	}
	return set
}

// removedLists returns the edges an amendment removed, sorted, with a "v:"
// prefix for validation-dependency removals so the two kinds are distinct.
func (s amendedEdgeSummary) removedLists() []string {
	var out []string
	for id := range s.refRemoved {
		out = append(out, id)
	}
	for id := range s.valRemoved {
		out = append(out, "v:"+id)
	}
	sort.Strings(out)
	return out
}

func amendedMark(amended bool) string {
	if amended {
		return "*"
	}
	return " "
}
