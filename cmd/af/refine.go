package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newRefineCmd creates the refine command for adding child nodes to a proof.
func newRefineCmd() *cobra.Command {
	var owner string
	var statement string
	var nodeType string
	var inference string
	var dir string
	var format string

	cmd := &cobra.Command{
		Use:   "refine <parent-id>",
		Short: "Add a child node to a claimed parent",
		Long: `Add a child node to a claimed parent node.

The parent node must be claimed by the owner specified with --owner.
Child IDs are auto-generated (e.g., 1.1, 1.2 for children of node 1).

Examples:
  af refine 1 --owner agent1 --statement "First subgoal"
  af refine 1 --owner agent1 -s "Case 1" --type case --inference local_assume
  af refine 1.1 -o agent1 -s "Deeper refinement" -T claim -i modus_ponens`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefine(cmd, args[0], owner, statement, nodeType, inference, dir, format)
		},
	}

	cmd.Flags().StringVarP(&owner, "owner", "o", "", "Agent/owner name (required, must match claim owner)")
	cmd.Flags().StringVarP(&statement, "statement", "s", "", "Child node statement (required)")
	cmd.Flags().StringVarP(&nodeType, "type", "T", "claim", "Child node type (claim/local_assume/local_discharge/case/qed)")
	cmd.Flags().StringVarP(&inference, "inference", "i", "assumption", "Inference type")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")

	return cmd
}

func runRefine(cmd *cobra.Command, parentIDStr, owner, statement, nodeTypeStr, inferenceStr, dir, format string) error {
	// Validate owner is not empty
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("owner is required")
	}

	// Validate statement is not empty
	if strings.TrimSpace(statement) == "" {
		return fmt.Errorf("statement is required and cannot be empty")
	}

	// Parse parent ID
	parentID, err := types.Parse(parentIDStr)
	if err != nil {
		return fmt.Errorf("invalid parent ID: %v", err)
	}

	// Validate node type
	if err := schema.ValidateNodeType(nodeTypeStr); err != nil {
		validTypes := []string{"claim", "local_assume", "local_discharge", "case", "qed"}
		return fmt.Errorf("invalid node type %q. Valid types: %s", nodeTypeStr, strings.Join(validTypes, ", "))
	}
	nodeType := schema.NodeType(nodeTypeStr)

	// Validate inference type
	if err := schema.ValidateInference(inferenceStr); err != nil {
		return fmt.Errorf("invalid inference type %q: %v", inferenceStr, err)
	}
	inferenceType := schema.InferenceType(inferenceStr)

	// Create service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to open proof: %v", err)
	}

	// Load state to determine next child ID
	st, err := svc.LoadState()
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "no events") || strings.Contains(errStr, "empty") ||
			strings.Contains(errStr, "no such file or directory") || strings.Contains(errStr, "does not exist") {
			return fmt.Errorf("proof not initialized. Run 'af init' first")
		}
		return fmt.Errorf("failed to load state: %v", err)
	}

	// Check if parent exists
	if st.GetNode(parentID) == nil {
		return fmt.Errorf("parent node %q does not exist", parentIDStr)
	}

	// Find next available child ID
	childNum := 1
	for {
		candidateID, err := parentID.Child(childNum)
		if err != nil {
			return fmt.Errorf("failed to generate child ID: %v", err)
		}
		if st.GetNode(candidateID) == nil {
			break
		}
		childNum++
	}

	childID, err := parentID.Child(childNum)
	if err != nil {
		return fmt.Errorf("failed to generate child ID: %v", err)
	}

	// Call RefineNode
	err = svc.RefineNode(parentID, owner, childID, nodeType, statement, inferenceType)
	if err != nil {
		// Provide helpful error messages
		if strings.Contains(err.Error(), "not claimed") {
			return fmt.Errorf("parent node is not claimed. Claim it first with 'af claim %s'", parentIDStr)
		}
		if strings.Contains(err.Error(), "owner does not match") {
			return fmt.Errorf("owner does not match the claim owner for node %s", parentIDStr)
		}
		return err
	}

	// Output result
	if format == "json" {
		result := map[string]interface{}{
			"success":   true,
			"parent_id": parentIDStr,
			"child_id":  childID.String(),
			"type":      nodeTypeStr,
			"statement": statement,
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %v", err)
		}
		cmd.Println(string(jsonBytes))
	} else {
		cmd.Printf("Node refined successfully.\n")
		cmd.Printf("  Parent: %s\n", parentIDStr)
		cmd.Printf("  Child:  %s\n", childID.String())
		cmd.Printf("  Type:   %s\n", nodeTypeStr)
		cmd.Printf("  Statement: %s\n", statement)
		cmd.Println("\nNext steps:")
		cmd.Printf("  af refine %s    - Add more children to this node\n", childID.String())
		cmd.Printf("  af status       - View proof status\n")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newRefineCmd())
}
