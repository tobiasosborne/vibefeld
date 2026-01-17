// Package main contains the af deps command implementation.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newDepsCmd creates the deps command for showing dependency graph.
func newDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps <node-id>",
		Short: "Show dependency graph for a node",
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
	nodeID, err := types.Parse(nodeIDStr)
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
		return fmt.Errorf("node %s not found", nodeIDStr)
	}

	// Build dependency info
	type depInfo struct {
		ID            string `json:"id"`
		Statement     string `json:"statement"`
		State         string `json:"epistemic_state"`
		IsBlocking    bool   `json:"is_blocking,omitempty"`
		DepType       string `json:"type"` // "reference" or "validation"
	}

	var deps []depInfo

	// Process reference dependencies
	for _, depID := range node.Dependencies {
		dep := st.GetNode(depID)
		info := depInfo{
			ID:      depID.String(),
			DepType: "reference",
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
		}
		if dep != nil {
			info.Statement = truncateString(dep.Statement, 50)
			info.State = string(dep.EpistemicState)
			// A validation dep is blocking if not validated/admitted
			if dep.EpistemicState != schema.EpistemicValidated && dep.EpistemicState != schema.EpistemicAdmitted {
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
			valDepStrs := make([]string, len(node.ValidationDeps))
			for i, d := range node.ValidationDeps {
				valDepStrs[i] = d.String()
			}
			result["validation_deps"] = valDepStrs
		}
		if len(node.Dependencies) > 0 {
			refDepStrs := make([]string, len(node.Dependencies))
			for i, d := range node.Dependencies {
				refDepStrs[i] = d.String()
			}
			result["reference_deps"] = refDepStrs
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		cmd.Println(string(jsonBytes))
	} else {
		// Text output
		cmd.Printf("Dependencies for node %s\n", nodeIDStr)
		cmd.Printf("Statement: %s\n", truncateString(node.Statement, 60))
		cmd.Printf("State: %s\n\n", node.EpistemicState)

		if len(deps) == 0 {
			cmd.Println("No dependencies")
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
				cmd.Println("Reference Dependencies:")
				for _, d := range refDeps {
					cmd.Printf("  %s [%s] - %s\n", d.ID, d.State, d.Statement)
				}
				cmd.Println()
			}

			if len(valDeps) > 0 {
				cmd.Println("Validation Dependencies (must be validated before accepting):")
				for _, d := range valDeps {
					status := d.State
					if d.IsBlocking {
						status += " (BLOCKING)"
					} else {
						status += " (satisfied)"
					}
					cmd.Printf("  %s [%s] - %s\n", d.ID, status, d.Statement)
				}
				cmd.Println()
			}

			if blockingCount > 0 {
				cmd.Printf("Status: BLOCKED - %d validation dependencies are unvalidated\n", blockingCount)
				cmd.Println("Cannot accept this node until all validation dependencies are validated.")
			} else if len(valDeps) > 0 {
				cmd.Println("Status: Ready - all validation dependencies are satisfied")
			}
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newDepsCmd())
}
