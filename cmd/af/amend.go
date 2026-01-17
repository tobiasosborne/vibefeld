package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newAmendCmd creates the amend command for correcting node statements.
func newAmendCmd() *cobra.Command {
	var owner string
	var statement string
	var dir string
	var format string

	cmd := &cobra.Command{
		Use:   "amend <node-id>",
		Short: "Amend a node's statement (prover correction)",
		Long: `Amend a node's statement to correct mistakes.

The amend command allows provers to correct the statement of a node they own.
This is useful for fixing typos, clarifying wording, or correcting errors
discovered before the node has been validated.

Requirements:
  - You must be the owner of the node (or provide --owner)
  - The node must be in 'pending' epistemic state (not yet validated/refuted)
  - The node must not be claimed by another agent

The original statement is preserved in the amendment history, which can be
viewed with 'af get <node-id> --full'.

Examples:
  af amend 1.1 --owner agent1 --statement "Corrected claim about X"
  af amend 1.2 -o agent1 -s "Fixed typo in the proof step"
  af amend 1.1 --owner agent1 --statement "Clarified statement" --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAmend(cmd, args[0], owner, statement, dir, format)
		},
	}

	cmd.Flags().StringVarP(&owner, "owner", "o", "", "Agent/owner name (required)")
	cmd.Flags().StringVarP(&statement, "statement", "s", "", "New statement text (required)")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")

	return cmd
}

func runAmend(cmd *cobra.Command, nodeIDStr, owner, statement, dir, format string) error {
	examples := render.GetExamples("af amend")

	// Validate owner is not empty
	if strings.TrimSpace(owner) == "" {
		return render.MissingFlagError("af amend", "owner", examples)
	}

	// Validate statement is not empty
	if strings.TrimSpace(statement) == "" {
		return render.MissingFlagError("af amend", "statement", examples)
	}

	// Parse node ID
	nodeID, err := types.Parse(nodeIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af amend", nodeIDStr, examples)
	}

	// Create service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to open proof: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("failed to check proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized. Run 'af init' first")
	}

	// Load state to get the original statement for output
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		return fmt.Errorf("node %q does not exist", nodeIDStr)
	}
	originalStatement := n.Statement

	// Perform the amendment
	err = svc.AmendNode(nodeID, owner, statement)
	if err != nil {
		// Provide helpful error messages
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("node %q does not exist", nodeIDStr)
		}
		if strings.Contains(err.Error(), "epistemic state") {
			return fmt.Errorf("cannot amend node %s: only nodes in 'pending' state can be amended", nodeIDStr)
		}
		if strings.Contains(err.Error(), "claimed by") {
			return fmt.Errorf("cannot amend node %s: %v", nodeIDStr, err)
		}
		return err
	}

	// Output result
	if format == "json" {
		result := map[string]interface{}{
			"success":            true,
			"node_id":            nodeIDStr,
			"previous_statement": originalStatement,
			"new_statement":      statement,
			"owner":              owner,
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		cmd.Println(string(jsonBytes))
	} else {
		cmd.Printf("Node %s amended successfully.\n", nodeIDStr)
		cmd.Printf("  Previous: %s\n", truncateString(originalStatement, 60))
		cmd.Printf("  New:      %s\n", truncateString(statement, 60))
		cmd.Printf("  Owner:    %s\n", owner)
		cmd.Println("\nNext steps:")
		cmd.Printf("  af get %s --full    - View node with amendment history\n", nodeIDStr)
		cmd.Printf("  af status           - View proof status\n")
	}

	return nil
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	rootCmd.AddCommand(newAmendCmd())
}
